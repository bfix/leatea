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
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	// global time source, incremented in epochs
	time int

	// pre-defined list of nodes (id, neighbors...)
	nodes = map[int]*Peer{
		1: NewPeer(1, 2, 4),
		2: NewPeer(2, 1, 3, 6),
		3: NewPeer(3, 2, 5, 7),
		4: NewPeer(4, 1, 5, 6),
		5: NewPeer(5, 3, 4),
		6: NewPeer(6, 2, 4, 7),
		7: NewPeer(7, 3, 6, 8),
		8: NewPeer(8, 7),
	}
)

// NewPeer creates a new node
func NewPeer(id int, nbs ...int) *Peer {
	// create neighbors map
	nb := make(map[int]struct{})
	for _, n := range nbs {
		nb[n] = struct{}{}
	}
	// create peer
	return &Peer{
		id:        id,
		tbl:       make(map[int]*Entry),
		neighbors: nb,
	}
}

// Peer is a participant in the ad-hoc network.
type Peer struct {
	id        int              // PeerID
	tbl       map[int]*Entry   // forward table
	neighbors map[int]struct{} // neighbors
}

// Forward as used in TEAch messages
type Forward struct {
	peer   int // target (route destination)
	hops   int // expected number of hops
	origin int // timestamp of origin
}

// String returns a human-readable representation
func (f *Forward) String() string {
	return fmt.Sprintf("{%d,%d,%d}", f.peer, f.hops, f.origin)
}

// Entry in forward table
type Entry struct {
	Forward
	next int // next hop
}

// NewEntry creates an entry with specified values
func NewEntry(id, next, hops, origin int) *Entry {
	return &Entry{
		Forward: Forward{
			peer:   id,
			hops:   hops,
			origin: origin,
		},
		next: next,
	}
}

// String returns a human-readable representation
func (e *Entry) String() string {
	return fmt.Sprintf("{%s,%d}", e.Forward.String(), e.next)
}

// Learn creates a new LEArn message
func (p *Peer) Learn() (m *LearnMsg) {
	// check for expired neighbors in the table
	for _, e := range p.tbl {
		if e.hops >= 0 && e.next == 0 && time-e.origin > 15 {
			fmt.Printf("Neighbor peer %d expired on %d\n", e.peer, p.id)
			// tag as "removed neighbor"
			e.hops = -2
			e.origin = time
			// remove dependent forward if next hop is neighbor
			for _, d := range p.tbl {
				if d.next == e.peer {
					// tag as "removed forward"
					d.hops = -1
					d.origin = time
				}
			}
		}
	}
	// add all known forwards to learn request
	list := make([]int, 0)
	for id := range p.tbl {
		list = append(list, id)
	}
	// add ourself
	list = append(list, p.id)
	// assemble message
	return &LearnMsg{p.id, list}
}

// add a neighbor node (when receiving a message from it)
func (p *Peer) addNeighbor(node int) {
	if entry, ok := p.tbl[node]; ok {
		// update existing entry
		entry.origin = time
	} else {
		// insert new entry
		p.tbl[node] = NewEntry(node, 0, 0, time)
	}
}

// HandleLearn processes received a LEArn message
func (p *Peer) HandleLearn(msg *LearnMsg) *TeachMsg {
	fmt.Printf("Peer %d wants to learn from %d\n", msg.sender, p.id)

	// add/update sender as neighbor
	p.addNeighbor(msg.sender)

	// collect entries for TEAch response
	resp := make([]*Forward, 0)
	for _, entry := range p.tbl {
		// add entry if not filtered
		add := index(entry.peer, msg.known) == -1
		// create forward for response
		forward := &Forward{entry.peer, entry.hops, entry.origin}
		switch entry.hops {
		// removed forward
		case -1:
			// forced add
			add = true
			// remove forward from table
			delete(p.tbl, entry.peer)
		// removed neighbor
		case -2:
			// tag as deleted neighbor
			entry.hops = -3
			// forced add
			add = true
		case -3:
			// do not add deleted neighbors (even when unfiltered)
			add = false
		}
		// add forward to response if required
		if add {
			resp = append(resp, forward)
		}
	}
	// return TEach message if response is not empty
	if len(resp) > 0 {
		return &TeachMsg{
			sender:    p.id,
			announces: resp,
		}
	}
	return nil
}

// HandleTeach processes an incoming TEAch message
func (p *Peer) HandleTeach(msg *TeachMsg) {
	fmt.Printf("Peer %d taught by peer %d\n", p.id, msg.sender)

	// add/update sender as neighbor
	p.addNeighbor(msg.sender)

	// for all forwards in message
	for _, announce := range msg.announces {
		// ignore forwards of ourself
		if announce.peer == p.id {
			continue
		}
		// get matching table entry for forward
		entry, ok := p.tbl[announce.peer]
		if !ok && announce.hops >= 0 {
			// not found: insert new entry
			p.tbl[announce.peer] = NewEntry(announce.peer, msg.sender, announce.hops+1, announce.origin)
			return
		}
		// skip if announce is older than entry
		if announce.origin < entry.origin {
			continue
		}
		// "removal" announce on active entry?
		if announce.hops < 0 && entry.hops >= 0 {
			s1 := entry.String()
			h1 := entry.hops
			// removed neighbor?
			if announce.hops == -2 {
				// update entry: tag as removed neighbor
				entry.hops = -2
				entry.next = 0
				entry.origin = announce.origin
				// remove dependent forward if next hop is removed neighbor
				for _, forward := range p.tbl {
					if forward.next == entry.peer {
						forward.hops = -1
						forward.origin = entry.origin
					}
				}
			} else if entry.next == msg.sender {
				// remove entry if next hop is sender
				entry.hops = -1
				entry.origin = announce.origin
			}
			fmt.Printf("MUT [%d<%d] %s --> %s on %s\n", p.id, msg.sender, s1, entry, announce)
			if h1 == -1 && entry.hops >= 0 {
				panic("resurrected removed forward")
			}
		} else if announce.hops+1 < entry.hops {
			// short route found
			entry.hops = announce.hops + 1
			entry.next = msg.sender
			entry.origin = announce.origin
		}
	}
}

// TeachMsg as a response to LEArn requests
type TeachMsg struct {
	sender    int
	announces []*Forward
}

// LearnMsg to periodically asked for new information
type LearnMsg struct {
	sender int
	known  []int
}

// run simulation
func main() {
	keyb := bufio.NewReader(os.Stdin)
	ui := func(flag bool) {
		if !flag {
			return
		}
		inp, _, _ := keyb.ReadLine()
		s := strings.Split(string(inp), " ")
		if len(s) > 0 {
			switch s[0] {
			case "k":
				if n, err := strconv.Atoi(s[1]); err == nil {
					delete(nodes, n)
				}
			}
		}
	}

	//------------------------------------------------------------------
	for time = 1; time < 100; time++ {
		fmt.Printf("Time %d -------------------------------------------\n", time)
		for _, sender := range nodes {
			learn := sender.Learn()
			fmt.Printf("Learn from %d: %v\n", learn.sender, learn.known)

			teaches := make([]*TeachMsg, 0)
			for id := range sender.neighbors {
				rcv := nodes[id]
				if rcv == nil {
					continue
				}
				if tm := rcv.HandleLearn(learn); tm != nil {
					teaches = append(teaches, tm)
				}
			}
			for _, teach := range teaches {
				fmt.Printf("Teach from %d: %v\n", teach.sender, teach.announces)
				for id := range nodes[teach.sender].neighbors {
					if id == teach.sender {
						continue
					}
					rcv := nodes[id]
					if rcv != nil {
						rcv.HandleTeach(teach)
					}
				}
			}
		}
		check()
		switch time {
		case 30:
			delete(nodes, 6)
			delete(nodes, 5)
		}
		ui(false)
	}
}

func check() {
	for _, p := range nodes {
		if p == nil {
			continue
		}
		fmt.Printf("Node %d: %v\n", p.id, p.tbl)

		for _, e := range p.tbl {
			if e.peer == p.id {
				fmt.Printf("self forward detected: peer %d, ft: %v, nb: %v\n", p.id, p.tbl, p.neighbors)
				panic("")
			}
			if e.next == p.id {
				fmt.Printf("forward to self detected: peer %d, ft: %v, nb: %v\n", p.id, p.tbl, p.neighbors)
				panic("")
			}
			if e.next != 0 {
				if _, ok := p.neighbors[e.next]; !ok {
					fmt.Printf("next %d not a neighbor(%v)\n", e.next, p.neighbors)
					panic("")
				} else {
					t := p.tbl[e.next]
					if t.hops < 0 && e.hops >= 0 {
						fmt.Printf("Peer %d, next %d (%s) removed on %s\n", p.id, e.next, e.String(), t.String())
						panic("")
					}
				}
			}
		}
	}
	total := 0
	failed := 0
	loops := 0
	ttl := len(nodes)
	for n1 := range nodes {
	targets:
		for n2 := range nodes {
			total++
			if n1 == n2 {
				continue
			}
			from := n1
			to := n2
			hops := 0
			var route []int
			for from != to && hops < ttl {
				route = append(route, from)
				hop, ok := nodes[from]
				if !ok {
					failed++
					fmt.Printf("%d -> %d failed: %v\n", n1, n2, route)
					panic("")
					//continue targets
				}
				n, ok := hop.tbl[to]
				if !ok {
					failed++
					continue targets
				}
				hops++
				if n.peer == to {
					continue targets
				}
				from = n.next
			}
			if hops == ttl {
				loops++
			}
		}
	}
	fmt.Printf("Total: %d, failed: %d, loops: %d\n", total, failed, loops)
}

func index(i int, list []int) int {
	for j, id := range list {
		if id == i {
			return j
		}
	}
	return -1
}
