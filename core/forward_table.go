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

// Entry in forward table
type Entry struct {
	Peer     *PeerID ``                // target node
	Hops     uint16  `size:"big"`      // number of hops to target
	NextHop  *PeerID `opt:"(WithHop)"` // next hop (optional)
	LastSeen *Time   ``                // last time seen
}

// WithHop returns true if next hop is set (used for serialization).
func (e *Entry) WithHop() bool {
	return e.Hops > 0
}

// Size of the binary representation
func (e *Entry) Size() uint {
	size := e.Peer.Size() + 10
	if e.Hops > 0 {
		size += e.NextHop.Size()
	}
	return size
}

// Clone entry for response
func (e *Entry) Clone() *Entry {
	return &Entry{
		Peer:     e.Peer,
		NextHop:  e.NextHop,
		Hops:     e.Hops,
		LastSeen: e.LastSeen,
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
	t.list[e.Peer.Key()] = e
}

// Filter returns a bloomfilter from all table entries (PeerID)
func (t *ForwardTable) Filter() *data.SaltedBloomFilter {
	t.RLock()
	defer t.RUnlock()
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
// sender as next hop.
func (t *ForwardTable) Candidates(m *LearnMsg) (list []*Entry) {
	t.Lock()
	defer t.Unlock()
	for _, e := range t.list {
		if !m.Filter.Contains(e.Peer.Bytes()) && !m.Sender().Equal(e.NextHop) {
			list = append(list, e.Clone())
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
			}
		} else {
			// not yet known: add to table
			t.list[key] = &Entry{
				Peer:     e.Peer,
				NextHop:  m.Sender(),
				Hops:     e.Hops + 1,
				LastSeen: TimeNow(),
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
