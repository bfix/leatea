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
// is forwarded in a teach message, the entry is removed from the table.
//----------------------------------------------------------------------

// Forward (target peer, distance/hops and age)
// Forwards are send by peers to their neighbors to inform them about
// target peers they know about (see TEAch message handling). If a
// received forward is not in the forward table of a peer, it is added
// with the sender as next hop and a hop count increased by 1. The age
// of the forward is preserved in the new table entry.
type Forward struct {
	// target node
	Peer *PeerID

	// expected number of hops to target
	// (0 = neighbor, -1 = removed neighbor)
	Hops int16 `size:"big"`

	// age of entry since creation (set when sending message)
	Age *Age
}

// Size returns the size of the binary representation (used to calculate
// size of TEAch message based on number of Forward entries)
func (f *Forward) Size() uint {
	var id *PeerID
	var age *Age
	return id.Size() + age.Size() + 2
}

//......................................................................

// Entry in forward table
type Entry struct {
	Forward

	// Next hop (nil for neighbors)
	NextHop *PeerID

	// Timestamp of the forward (route)
	// It is the time the target was seen by its neighbor from which
	// this route originated.
	Origin *Time

	// Entry changed but not forwarded yet
	// It is set to true of new and changed entries. It flags forwards
	// that the node learned that have not be been send in a TEAch yet.
	Pending bool
}

// Target returns the Forward for a table entry.
// The age of the entry is calculated relative to TimeNow()
func (e *Entry) Target() *Forward {
	return &Forward{
		Peer: e.Peer,
		Hops: e.Hops,
		Age:  e.Origin.Age(),
	}
}

// DirectEntry creates a pending entry for a neighbor.
func DirectEntry(n *PeerID) *Entry {
	return &Entry{
		Forward: Forward{
			Peer: n,
			Hops: 0,
		},
		NextHop: nil,
		Origin:  TimeNow(),
		Pending: true,
	}
}

//----------------------------------------------------------------------
// FowardTable holds a list of entries (full forwards) to all targets
// learned from the leatea protocol.
//----------------------------------------------------------------------

// ForwardTable is a map of entries with key "target"
type ForwardTable struct {
	sync.RWMutex
	self     *PeerID           // reference to ourself
	list     map[string]*Entry // forward table
	listener Listener          // listener for events

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

// AddNeighbor to forward table:
// A (new) neighbor was seen being active (we received a message from it),
// so the entry for the neighbor is either added to or updated in the table.
func (t *ForwardTable) AddNeighbor(n *PeerID) {
	t.Lock()
	defer t.Unlock()
	// check if entry exists
	if e, ok := t.list[n.Key()]; ok {
		// yes: update timestamp
		e.Origin = TimeNow()
		e.Pending = true
		return
	}
	// new neighbor: insert new entry into table
	t.list[n.Key()] = DirectEntry(n)
}

// Drop entries related to a target peer:
// The timestamp is the creation time of the root removal; it is
// the current time if we are the originator of the deletion.
// N.B.: This is an internal method that must be called in a locked context.
func (t *ForwardTable) drop(n *PeerID, ts *Time) {
	// get table entry for target
	entry, ok := t.list[n.Key()]
	if !ok {
		// not found
		return
	}
	// already removed?
	if entry.Hops < 0 {
		// yes
		return
	}
	// is the existing entry newer than the removal request?
	if ts.Before(entry.Origin) {
		// yes
		return
	}
	// flag entry as removed
	entry.Hops = -1
	entry.Origin = ts
	entry.Pending = true

	// check for dropped neighbor
	if entry.NextHop == nil && entry.Hops == 0 {
		// notify listener
		if t.listener != nil {
			t.listener(&Event{
				Type: EvNeighborExpired,
				Peer: t.self,
				Ref:  entry.Peer,
			})
		}
		// drop forwards depending on removed neighbor
		for _, dep := range t.list {
			// skip neighbors and flagged entries
			if dep.NextHop == nil || dep.Hops < 0 {
				continue
			}
			t.drop(dep.Peer, ts)
		}
	} else {
		// notify listener we removed a dependent forward
		if t.listener != nil {
			t.listener(&Event{
				Type: EvForwardRemoved,
				Peer: t.self,
				Ref:  entry.Peer,
			})
		}
	}
}

// Cleanup forward table and flag expired neighbors (and their dependencies)
// for removal. The actual deletion of the entry in the table happens after
// the removed entry was broadcasted in a TEAch message.
func (t *ForwardTable) Cleanup() {
	t.RLock()
	defer t.RUnlock()

	// remove expired neighbors
	for _, entry := range t.list {
		// is entry a neighbor?
		if entry.NextHop != nil {
			// no:
			continue
		}
		// is entry pending for deletion?
		if entry.Hops < 0 {
			// yes:
			continue
		}
		// has the entry expired?
		if !entry.Origin.Expired(time.Duration(cfg.TTLEntry) * time.Second) {
			// no:
			continue
		}
		// Drop neighbor:
		// We are the origin of the removal request, so use current time.
		t.drop(entry.Peer, TimeNow())
	}
}

// Filter returns a bloomfilter from all table entries (PeerID).
// Remove expired entries first.
func (t *ForwardTable) Filter() *data.SaltedBloomFilter {
	// clean-up first
	t.Cleanup()

	// create bloomfilter
	t.Lock()
	defer t.Unlock()
	salt := RndUInt32()
	n := len(t.list) + 2
	fpr := 1. / float64(n)
	pf := data.NewSaltedBloomFilter(salt, n, fpr)

	// add all table entries that are not tagged for deletion
	for _, entry := range t.list {
		// skip removed entry
		if entry.Hops < 0 {
			continue
		}
		// add entry to filter
		pf.Add(entry.Peer.Bytes())
	}
	// add ourself to the filter (can't learn about myself from others)
	pf.Add(t.self.Bytes())
	return pf
}

// Candiates returns a list of table entries that are not filtered out by the
// bloomfilter contained in the LEArn message.
// Candiates also can't have sender as next hop. (TODO: check!!)
// Pending entries (updated but not forwarded yet) are collected if there is
// space for them in the result list.
func (t *ForwardTable) Candidates(m *LEArnMsg) (list []*Forward) {
	t.Lock()
	defer t.Unlock()

	//------------------------------------------------------------------
	// (1) collect unfiltered entries
	//------------------------------------------------------------------
	fList := make([]*Entry, 0)
	for _, entry := range t.list {
		// entry filtered out?
		if m.Filter.Contains(entry.Peer.Bytes()) {
			continue
		}
		// skip if sender is next hop
		if m.Sender().Equal(entry.NextHop) {
			continue
		}
		// add entr to list
		fList = append(fList, entry)
	}
	// sort list by ascending number of hops
	sort.Slice(fList, func(i, j int) bool {
		return fList[i].Hops < fList[j].Hops
	})
	// if list limit is reached (or surpassed), trim results
	if len(fList) >= cfg.MaxTeachs {
		fList = fList[:cfg.MaxTeachs]
	}
	// append results to candidate list
	for _, entry := range fList {
		// entry no longer dirty
		entry.Pending = false
		list = append(list, entry.Target())
		// delete removed entries
		if entry.Hops < 0 {
			delete(t.list, entry.Peer.Key())
		}
	}

	//------------------------------------------------------------------
	// (2) collect pending entries (if more space is available in TEAch)
	//------------------------------------------------------------------
	if len(list) < cfg.MaxTeachs {
		// keep list of pending entries
		var pList []*Entry
		for _, entry := range t.list {
			if entry.Pending {
				pList = append(pList, entry)
			}
		}
		// append pending entries to candidate list
		if len(pList) > 0 {
			// sort list by ascending hops (deleted first)
			sort.Slice(pList, func(i, j int) bool {
				return pList[i].Hops < pList[j].Hops
			})
			// append best entries to candidates list
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

// Learn from announcements in a TEAch message
func (t *ForwardTable) Learn(m *TEAchMsg) {
	t.Lock()
	defer t.Unlock()

	// process all announcements
	sender := m.Sender()
	for _, announce := range m.Announce {
		// ignore announcements about ourself
		if announce.Peer.Equal(t.self) {
			continue
		}
		// get the timestamp of the announcement
		origin := TimeFromAge(announce.Age)

		// get corresponding forward entry
		key := announce.Peer.Key()
		if entry, ok := t.list[key]; ok {
			// entry exists in the forward table:

			// out-dated announcement?
			if origin.Before(entry.Origin) {
				// yes: ignore old information
				continue
			}
			// check for update
			if announce.Hops < 0 {
				// "delete" announcement: check for impact
				if !sender.Equal(entry.NextHop) {
					// distinct route: preserve entry
					continue
				}
				// drop forward
				t.drop(entry.Peer, origin)
			} else if entry.Hops > announce.Hops+1 {
				// update with shorter path
				entry.Hops = announce.Hops + 1
				entry.NextHop = sender
				entry.Origin = origin
				entry.Pending = true
			}
		} else {
			// entry not yet known: add new entry to table
			t.list[key] = &Entry{
				Forward: Forward{
					Peer: announce.Peer,
					Hops: announce.Hops + 1,
				},
				NextHop: m.Sender(),
				Origin:  origin,
				Pending: true,
			}
		}
	}
}

// Forward returns the peerid of the next hop to target and the number of
// expected hops along the route.
func (t *ForwardTable) Forward(target *PeerID) (*PeerID, int) {
	t.RLock()
	defer t.RUnlock()
	// lookup entry in table
	if entry, ok := t.list[target.Key()]; ok {
		// ignore removed entries
		if entry.Hops < 0 {
			return nil, 0
		}
		// return forward information
		return entry.NextHop, int(entry.Hops) + 1
	}
	// target not in table
	return nil, 0
}

// NumForwards returns the number of (active) targets in the forward table
func (t *ForwardTable) NumForwards() (count int) {
	t.RLock()
	defer t.RUnlock()
	// count number of active forwards (including neighbors)
	for _, entry := range t.list {
		if entry.Hops >= 0 {
			count++
		}
	}
	return
}

// Return a list of active direct neighbors
func (t *ForwardTable) Neighbors() (list []*PeerID) {
	t.RLock()
	defer t.RUnlock()
	// collect neighbors from the table
	for _, entry := range t.list {
		if entry.NextHop == nil && entry.Hops == 0 {
			list = append(list, entry.Peer)
		}
	}
	return
}
