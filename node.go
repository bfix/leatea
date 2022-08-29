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

package main

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/bfix/gospel/data"
)

type Entry struct {
	peer     string
	hops     int
	nextHop  string
	lastSeen time.Time
}

func (e *Entry) Clone() *Entry {
	return &Entry{
		peer:     e.peer,
		hops:     e.hops,
		nextHop:  e.nextHop,
		lastSeen: e.lastSeen,
	}
}

type ForwardTable map[string]*Entry

type Node struct {
	netw    *Network
	local   string
	rt      ForwardTable
	pos     *Position
	epoch   int
	learned int
}

func NewNode(n *Network, pos *Position) *Node {
	return &Node{
		netw:    n,
		local:   fmt.Sprintf("Node #%d", nextID()),
		rt:      make(map[string]*Entry),
		pos:     pos,
		epoch:   rand.Intn(epoch),
		learned: 0,
	}
}

func (n *Node) Receive(msg Message) {
	// if the sender is not known yet, add it as direct neighbor to the
	// forward table.
	sender := msg.Sender().local
	if _, ok := n.rt[sender]; !ok {
		n.rt[sender] = &Entry{
			peer:     sender,
			nextHop:  "",
			hops:     0,
			lastSeen: time.Now(),
		}
		n.learned++
	}
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
		var candidates []*Entry
		for peer, e := range n.rt {
			if !m.pf.Contains([]byte(peer)) && m.sender.local != e.nextHop {
				candidates = append(candidates, e.Clone())
			}
		}
		// in case we have candidates...
		if len(candidates) > 0 {
			// sort them by ascending hops
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].hops < candidates[j].hops
			})
			// trim list if we have too many candidates
			if len(candidates) > maxTeach {
				candidates = candidates[:maxTeach]
			}
			// assemble and send TEACH message
			teach := new(TeachMsg)
			teach.sender = n
			teach.announce = candidates
			n.netw.Broadcast(teach)
		}

	//------------------------------------------------------------------
	// TEACH message received
	//------------------------------------------------------------------
	case MSG_TEACH:
		m, _ := msg.(*TeachMsg)
		// check if an entry in the announcement is already known
		// (happens in unrequested teach messages)
		for _, e := range m.announce {
			fwt, ok := n.rt[e.peer]
			if ok {
				// already known: shorter path?
				if fwt.hops > e.hops+1 {
					// update with shorter path
					fwt.hops = e.hops + 1
					fwt.nextHop = m.sender.local
					fwt.lastSeen = time.Now()
					n.learned++
				}
			} else {
				// not yet known: add to table
				n.rt[e.peer] = &Entry{
					peer:     e.peer,
					nextHop:  m.sender.local,
					hops:     e.hops + 1,
					lastSeen: time.Now(),
				}
				n.learned++
			}
		}
	}
}

func (n *Node) Epoch() int {
	// has our learn interval expired?
	if n.epoch == 0 {
		// yes: send out our own learn message
		pf := data.NewBloomFilter(1000, 1e-3)
		pf.Add([]byte(n.local))
		for id := range n.rt {
			pf.Add([]byte(id))
		}
		msg := new(LearnMsg)
		msg.sender = n
		msg.pf = pf
		n.netw.Broadcast(msg)
		// start new learn interval
		n.epoch = epoch
	} else {
		n.epoch--
	}
	// return number of learned entries
	res := n.learned
	n.learned = 0
	return res
}

func (n *Node) String() string {
	return fmt.Sprintf("Node{%s: %v}", n.local, n.rt)
}
