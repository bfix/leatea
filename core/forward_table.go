//----------------------------------------------------------------------
// This file is part of leatea-routing.
// Copyright (C) 2022 Bernd Fix >Y<
//
// leatea-routing is free software: you can redistribute it and/or modify it
// under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License,
// or (at your option) any later version.
//
// leatea-routing is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
// Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
//
// SPDX-License-Identifier: AGPL3.0-or-later
//----------------------------------------------------------------------

package core

import (
	"sort"
	"sync"
	"time"

	"github.com/bfix/gospel/data"
)

//----------------------------------------------------------------------
// Forwarding table: each peer has a forwarding table with entries for
// all other peers it learned about over time. The entry specifies the
// peer ID of the other peer, the next hop on the route to the target,
// the number of hops to reach the target and a timestamp when the peer
// was last seen in the network. A direct neighbor (within broadcast
// range) has no next hop and a hop count of 0 in the table.
//
// If an entry is removed (because the neighbor expired), the hop
// count is set to -1 to indicated a deleted entry. Once such entry
// is forwarded in a teach message, it is removed from the table.
//----------------------------------------------------------------------

// Forward (target peerid and distance/hops)
type Forward struct {
	// target node
	Peer *PeerID

	// number of hops to target
	// (0 = neighbor, -1 = removed neighbor)
	Hops int16 `size:"big"`
}

// Size returns the size of the binary representation (used to calculate
// size of TEACH message based on number of Forward entries)
func (f *Forward) Size() uint {
	var id *PeerID
	return id.Size() + 2
}

//......................................................................

// Entry in forward table
type Entry struct {
	Forward
	NextHop  *PeerID // next hop (nil for neighbors)
	LastSeen *Time   // last time seen
	Pending  bool    // entry changed but not forwarded yet
}

// Equal returns true if the base attributes of the entries are the same
func (e *Entry) Equal(e2 *Entry) bool {
	return e.Peer.Equal(e2.Peer) && e.Hops == e2.Hops && e.NextHop.Equal(e2.NextHop)
}

// Forward entry for response (TEACH message)
func (e *Entry) Target() *Forward {
	return &Forward{
		Peer: e.Peer,
		Hops: e.Hops,
	}
}

//----------------------------------------------------------------------

// ForwardTable is a map of entries with key "target"
type ForwardTable struct {
	sync.RWMutex
	self *PeerID
	list map[string]*Entry
}

// NewForwardTable creates an empty table
func NewForwardTable(self *PeerID) *ForwardTable {
	return &ForwardTable{
		self: self,
		list: make(map[string]*Entry),
	}
}

// Reset routing table
func (t *ForwardTable) Reset() {
	t.Lock()
	defer t.Unlock()
	t.list = make(map[string]*Entry)
}

// AddNeighbor entry to forward table
func (t *ForwardTable) AddNeighbor(n *PeerID) {
	t.Lock()
	defer t.Unlock()
	// check for changes if entry exists
	if e, ok := t.list[n.Key()]; ok {
		// no change, but update timestamp
		e.LastSeen = TimeNow()
		return
	}
	// insert new entry into table
	t.list[n.Key()] = &Entry{
		Forward: Forward{
			Peer: n,
			Hops: 0,
		},
		NextHop:  nil,
		LastSeen: TimeNow(),
		Pending:  true,
	}
}

// Cleanup forward table and remove expired neighbors and their dependencies.
func (t *ForwardTable) Cleanup(cb Listener) {
	t.Lock()
	defer t.Unlock()
	// remove expired neighbors
	nList := make(map[string]struct{})
	for _, e := range t.list {
		if e.NextHop == nil && e.Hops != -1 && e.LastSeen.Expired(time.Duration(cfg.TTLEntry)*time.Second) {
			nList[e.Peer.Key()] = struct{}{}
			e.Hops = -1
			if cb != nil {
				cb(&Event{
					Type: EvNeighborExpired,
					Peer: t.self,
					Ref:  e.Peer,
				})
			}
		}
	}
	// remove forwards depending on removed neighbors
	for _, e := range t.list {
		if e.NextHop != nil && e.Hops != -1 {
			if _, ok := nList[e.NextHop.Key()]; ok {
				e.Hops = -1
				if cb != nil {
					cb(&Event{
						Type: EvForwardRemoved,
						Peer: t.self,
						Ref:  e.Peer,
					})
				}
			}
		}
	}
}

// Filter returns a bloomfilter from all table entries (PeerID).
// Remove expired entries first.
func (t *ForwardTable) Filter(cb Listener) *data.SaltedBloomFilter {
	// cleanup first
	t.Cleanup(cb)

	// generate bloomfilter
	t.Lock()
	defer t.Unlock()
	salt := RndUInt32()
	n := len(t.list) + 2
	fpr := 1. / float64(n)
	pf := data.NewSaltedBloomFilter(salt, n, fpr)
	for _, e := range t.list {
		pf.Add(e.Peer.Bytes())
	}
	pf.Add(t.self.Bytes())
	return pf
}

// Candiates from the table not filtered out. Candiates also can't have
// sender as next hop. Pending entries (updated but not forwarded yet)
// are collected if there is space for them in the result list
func (t *ForwardTable) Candidates(m *LearnMsg) (list []*Forward) {
	t.Lock()
	defer t.Unlock()

	//------------------------------------------------------------------
	// (1) collect unfiltered entries
	var fList []*Entry
	for _, e := range t.list {
		if !m.Filter.Contains(e.Peer.Bytes()) && !m.Sender().Equal(e.NextHop) {
			fList = append(fList, e)
		}
	}
	// sort them by ascending hops
	sort.Slice(fList, func(i, j int) bool {
		return fList[i].Hops < fList[j].Hops
	})
	// if list limit is reached (or surpassed), trim results
	if len(fList) >= cfg.MaxTeachs {
		fList = fList[:cfg.MaxTeachs]
	}
	// append results to output list
	for _, e := range fList {
		e.Pending = false
		list = append(list, e.Target())
		// delete removed entries
		if e.Hops < 0 {
			delete(t.list, e.Peer.Key())
		}
	}

	//------------------------------------------------------------------
	// (2) collect pending entries (if more space is available in TEACH)
	if len(list) < cfg.MaxTeachs {
		var pList []*Entry
		for _, e := range t.list {
			if e.Pending {
				pList = append(pList, e)
			}
		}
		// append pennding entries
		if len(pList) > 0 {
			// sort them by ascending hops (deleted first)
			sort.Slice(pList, func(i, j int) bool {
				return pList[i].Hops < pList[j].Hops
			})
			// append to result list
			n := cfg.MaxTeachs - len(list)
			if n > len(pList) {
				n = len(pList)
			}
			for i := 0; i < n; i++ {
				e := pList[i]
				e.Pending = false
				list = append(list, e.Target())
				// delete removed entries
				if e.Hops < 0 {
					delete(t.list, e.Peer.Key())
				}
			}
		}
	}
	return
}

// Learn from announcements
func (t *ForwardTable) Learn(m *TeachMsg) {
	t.Lock()
	defer t.Unlock()

	// process all announcements
	sender := m.Sender()
	for _, e := range m.Announce {
		// ignore announcements about ourself
		if e.Peer.Equal(t.self) {
			continue
		}
		// get corresponding forward entry
		key := e.Peer.Key()
		f, ok := t.list[key]
		if ok {
			// entry found; check for "delete" announcement
			if e.Hops < 0 {
				// entry tagged as removed.
				f.Hops = -1
				f.Pending = true
			} else if f.Hops > e.Hops+1 {
				// update with shorter path
				f.Hops = e.Hops + 1
				f.NextHop = sender
				f.Pending = true
			}
		} else {
			// not yet known: add to table
			t.list[key] = &Entry{
				Forward: Forward{
					Peer: e.Peer,
					Hops: e.Hops + 1,
				},
				NextHop: m.Sender(),
				Pending: true,
			}
		}
	}
}

// Forward returns the peerid of the next hop to target and the number of
// expected hops on the route.
func (t *ForwardTable) Forward(target *PeerID) (*PeerID, int) {
	t.RLock()
	defer t.RUnlock()
	f, ok := t.list[target.Key()]
	if !ok {
		return nil, 0
	}
	return f.NextHop, int(f.Hops) + 1
}
