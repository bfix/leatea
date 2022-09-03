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
	"log"
	"sort"
	"sync"

	"github.com/bfix/gospel/data"
)

//----------------------------------------------------------------------
// Forwarding table: each peer has a forwarding table with entries for
// all other peers it learned about over time. The entry specifies the
// peer ID of the other peer, the next hop on the route to the target,
// the number of hops to reach the target and a timestamp when the peer
// was last seen in the network. A direct neighbor (within broadcast
// range) has no next hop and a hop count of 0 in the table.
//----------------------------------------------------------------------

// Forward (target peerid and distance/hops)
type Forward struct {
	Peer *PeerID ``           // target node
	Hops uint16  `size:"big"` // number of hops to target
}

// Size returns the size of the binary representation
func (f *Forward) Size() uint {
	return 34
}

//......................................................................

// Entry in forward table
type Entry struct {
	Forward
	NextHop  *PeerID // next hop (nil for neighbors)
	LastSeen *Time   // last time seen
	Pending  bool    // entry changed but not forwarded
}

// WithHop returns true if next hop is set (used for serialization).
func (e *Entry) WithHop() bool {
	return e.Hops > 0
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

// Add entry to forward table
func (t *ForwardTable) Add(e *Entry) {
	t.Lock()
	defer t.Unlock()
	key := e.Peer.Key()
	// check for changes if entry exists
	if e2, ok := t.list[key]; ok && e.Equal(e2) {
		// no change
		e2.LastSeen = TimeNow()
		return
	}
	// insert/update into table
	e.Pending = true
	e.LastSeen = TimeNow()
	t.list[key] = e
}

// Cleanup forward table and remove expired neighbors and their dependencies.
func (t *ForwardTable) Cleanup() {
	t.Lock()
	defer t.Unlock()
	// remove expired neighbors
	nList := make(map[string]struct{})
	for k, e := range t.list {
		if e.NextHop == nil && e.LastSeen.Expired(ttlEntry) {
			nList[e.Peer.Key()] = struct{}{}
			delete(t.list, k)
			log.Printf("Neighbor %s of %s expired (%s)\n", e.Peer, t.self, e.LastSeen)
		}
	}
	// remove forwards depending on removed neighbors
	for k, e := range t.list {
		if e.NextHop != nil {
			if _, ok := nList[e.NextHop.Key()]; ok {
				delete(t.list, k)
				log.Printf("Target %s on %s removed\n", e.Peer, t.self)
			}
		}
	}
}

// Filter returns a bloomfilter from all table entries (PeerID).
// Remove expired entries first.
func (t *ForwardTable) Filter() *data.SaltedBloomFilter {
	// cleanup first
	t.Cleanup()

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

	// collect unfiltered entries
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
	// if list limit is reached, return results.
	if len(fList) >= maxTeachs {
		for _, e := range fList[:maxTeachs] {
			e.Pending = false
			list = append(list, e.Target())
		}
		return
	}

	// collect pending entries
	var pList []*Entry
	for _, e := range t.list {
		if e.Pending {
			pList = append(pList, e)
		}
	}
	// append pennding entries
	if len(pList) > 0 {
		// sort them by ascending hops
		sort.Slice(pList, func(i, j int) bool {
			return pList[i].Hops < pList[j].Hops
		})
		// append to result list
		n := maxTeachs - len(list)
		if n > len(pList) {
			n = len(pList)
		}
		for i := 0; i < n; i++ {
			e := pList[i]
			e.Pending = false
			list = append(list, e.Target())
		}
	}
	return
}

// Learn from announcements
func (t *ForwardTable) Learn(m *TeachMsg) {
	t.Lock()
	defer t.Unlock()
	for _, e := range m.Announce {
		if e.Peer.Equal(t.self) {
			continue
		}
		key := e.Peer.Key()
		fwt, ok := t.list[key]
		if ok {
			// already known: shorter path?
			if fwt.Hops > e.Hops+1 {
				// update with shorter path
				fwt.Hops = e.Hops + 1
				fwt.NextHop = m.Sender()
				fwt.LastSeen = TimeNow()
				fwt.Pending = true
			}
		} else {
			// not yet known: add to table
			t.list[key] = &Entry{
				Forward: Forward{
					Peer: e.Peer,
					Hops: e.Hops + 1,
				},
				NextHop:  m.Sender(),
				LastSeen: TimeNow(),
				Pending:  true,
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
