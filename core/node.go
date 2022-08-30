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
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bfix/gospel/data"
)

type Entry struct {
	Peer     *PeerID
	NextHop  *PeerID
	Hops     uint16 `size:"big"`
	LastSeen *Time
}

func (e *Entry) Clone() *Entry {
	return &Entry{
		Peer:     e.Peer,
		NextHop:  e.NextHop,
		Hops:     e.Hops,
		LastSeen: e.LastSeen,
	}
}

type ForwardTable struct {
	sync.RWMutex
	list map[string]*Entry
}

func NewForwardTable() *ForwardTable {
	return &ForwardTable{
		list: make(map[string]*Entry),
	}
}

func (t *ForwardTable) Add(e *Entry) {
	t.Lock()
	defer t.Unlock()
	t.list[e.Peer.Key()] = e
}

func (t *ForwardTable) Filter() *data.BloomFilter {
	t.RLock()
	defer t.RUnlock()
	pf := data.NewBloomFilter(1000, 1e-3)
	for _, e := range t.list {
		pf.Add(e.Peer.Bytes())
	}
	return pf
}

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

type Node struct {
	prv  *PeerPrivate
	pub  *PeerID
	rt   *ForwardTable
	recv chan Message
	send chan Message
}

func NewNode(prv *PeerPrivate, in, out chan Message) *Node {
	return &Node{
		prv:  prv,
		pub:  prv.Public(),
		rt:   NewForwardTable(),
		recv: in,
		send: out,
	}
}

func (n *Node) PeerID() *PeerID {
	return n.pub
}

func (n *Node) broadcast(msg Message) {
	go func() {
		n.send <- msg
	}()
}

func (n *Node) Forwards() int {
	return len(n.rt.list)
}

func (n *Node) Run() {

	learn := time.NewTicker(10 * time.Second)
	for {
		select {
		case <-learn.C:
			// send out our own learn message
			pf := n.rt.Filter()
			pf.Add(n.pub.Bytes())
			msg := new(LearnMsg)
			msg.Sender_ = n.pub
			msg.Filter = pf
			n.broadcast(msg)
		case msg := <-n.recv:
			go n.Receive(msg)
		}
	}
}

func (n *Node) Receive(msg Message) {
	// add the sender as direct neighbor to the
	// forward table.
	sender := msg.Sender()
	e := &Entry{
		Peer:     sender,
		NextHop:  nil,
		Hops:     0,
		LastSeen: TimeNow(),
	}
	n.rt.Add(e)

	// handle received message
	switch msg.Type() {

	//------------------------------------------------------------------
	// LEARN message received
	//------------------------------------------------------------------
	case MSG_LEARN:
		m, _ := msg.(*LearnMsg)
		// build a list of candidate entries for teaching:
		// candidates are not included in the learn filter
		// and don't have the learner as next hop.
		if candidates := n.rt.Candidates(m); len(candidates) > 0 {
			// sort them by ascending hops
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].Hops < candidates[j].Hops
			})
			// trim list if we have too many candidates
			if len(candidates) > maxTeachs {
				candidates = candidates[:maxTeachs]
			}
			// assemble and send TEACH message
			teach := new(TeachMsg)
			teach.Sender_ = n.pub
			teach.Announce = candidates
			n.broadcast(teach)
		}

	//------------------------------------------------------------------
	// TEACH message received
	//------------------------------------------------------------------
	case MSG_TEACH:
		m, _ := msg.(*TeachMsg)
		// learn new peers
		n.rt.Learn(m)
	}
}

func (n *Node) String() string {
	return fmt.Sprintf("Node{%s: %v}", n.pub.Short(), n.rt)
}
