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

// Entry in forward table
type Entry struct {
	Peer     *PeerID ``                // target node
	Hops     uint16  `size:"big"`      // number of hops
	NextHop  *PeerID `opt:"(WithHop)"` // next hop (optional)
	LastSeen *Time   ``                // last time seen
}

// WithHop returns true if next hop is set
func (e *Entry) WithHop() bool {
	return e.Hops > 0
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

// ForwardTable is a map of entries with key "target"
type ForwardTable struct {
	sync.RWMutex
	list map[string]*Entry
}

// NewForwardTable creates an empty table
func NewForwardTable() *ForwardTable {
	return &ForwardTable{
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
func (t *ForwardTable) Filter() *data.BloomFilter {
	t.RLock()
	defer t.RUnlock()
	pf := data.NewBloomFilter(1000, 1e-3)
	for _, e := range t.list {
		pf.Add(e.Peer.Bytes())
	}
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
		fwt, ok := t.list[e.Peer.Key()]
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
			t.list[e.Peer.Key()] = &Entry{
				Peer:     e.Peer,
				NextHop:  m.Sender(),
				Hops:     e.Hops + 1,
				LastSeen: TimeNow(),
			}
		}
	}
}
