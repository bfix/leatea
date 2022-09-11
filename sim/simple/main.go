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
	time int // global time source

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
	numNodes = len(nodes)
)

func NewPeer(id int, nbs ...int) *Peer {
	nb := make(map[int]struct{})
	for _, n := range nbs {
		nb[n] = struct{}{}
	}
	return &Peer{
		id: id,
		ft: make(map[int]*Entry),
		nb: nb,
	}
}

type Peer struct {
	id int
	ft map[int]*Entry
	nb map[int]struct{}
}

type Entry struct {
	peer int
	next int
	hops int
	ts   int
}

func (e *Entry) String() string {
	return fmt.Sprintf("{%d,%d,%d,%d}", e.peer, e.next, e.hops, e.ts)
}

func (p *Peer) Learn() (m *LearnMsg) {
	for _, e := range p.ft {
		if e.next == 0 && time-e.ts > 15 {
			fmt.Printf("Neighbor peer %d expired on %d\n", e.peer, p.id)
			e.hops = -2
			e.ts = time
			for _, d := range p.ft {
				if d.next == e.peer {
					d.hops = -1
					d.ts = time
				}
			}
		}
	}
	list := make([]int, 0)
	for id := range p.ft {
		list = append(list, id)
	}
	list = append(list, p.id)
	return &LearnMsg{p.id, list}
}

func (p *Peer) addNeighbor(n int) {
	if e, ok := p.ft[n]; ok {
		e.ts = time
	} else {
		e := &Entry{n, 0, 0, time}
		p.ft[n] = e
	}
}

func (p *Peer) HandleLearn(m *LearnMsg) *TeachMsg {
	fmt.Printf("Peer %d learning from %d\n", p.id, m.sender)
	p.addNeighbor(m.sender)
	list := make([]*Entry, 0)
	for _, e := range p.ft {
		el := &Entry{e.peer, e.next, e.hops, e.ts}
		add := index(e.peer, m.known) == -1
		switch e.hops {
		case -1:
			add = true
			delete(p.ft, e.peer)
		case -2:
			e.hops = -3
			el.hops = -2
			add = true
		case -3:
			add = false
		}
		if add {
			list = append(list, el)
		}
	}
	if len(list) > 0 {
		return &TeachMsg{
			sender:   p.id,
			announce: list,
		}
	}
	return nil
}

func (p *Peer) HandleTeach(tm *TeachMsg) {
	fmt.Printf("Peer %d taught by peer %d\n", p.id, tm.sender)
	p.addNeighbor(tm.sender)
	for _, e := range tm.announce {
		if e.peer == p.id {
			continue
		}
		f, ok := p.ft[e.peer]
		if ok {
			if f.ts > e.ts {
				continue
			}
			if e.hops < 0 && f.hops >= 0 {
				s1 := f.String()
				h1 := f.next
				if f.peer == e.peer {
					if e.next == 0 {
						f.hops = -2
						f.next = 0
						f.ts = e.ts
						for _, g := range p.ft {
							if g.next == e.peer {
								g.hops = -1
								g.ts = e.ts
							}
						}
					} else if f.next == tm.sender {
						f.hops = -1
						f.ts = e.ts
					}
				}
				fmt.Printf("MUT [%d<%d] %s --> %s on %s\n", p.id, tm.sender, s1, f, e)
				if h1 == -1 && f.next != -1 {
					panic("resurrected deletion")
				}
			} else if e.hops+1 < f.hops {
				f.hops = e.hops + 1
				f.next = tm.sender
				f.ts = e.ts
			}
		} else {
			p.ft[e.peer] = &Entry{e.peer, tm.sender, e.hops + 1, e.ts}
		}
	}
}

type TeachMsg struct {
	sender   int
	announce []*Entry
}

type LearnMsg struct {
	sender int
	known  []int
}

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
	nextLearner := 0
	for time = 1; time < 120; time++ {
		fmt.Printf("Time %d -------------------------------------------\n", time)

		var sender *Peer
		var ok bool
		for !ok {
			sender, ok = nodes[nextLearner+1]
			nextLearner = (nextLearner + 1) % numNodes
		}
		learn := sender.Learn()
		fmt.Printf("Learn from %d: %v\n", learn.sender, learn.known)

		teaches := make([]*TeachMsg, 0)
		for id := range sender.nb {
			rcv := nodes[id]
			if rcv == nil {
				continue
			}
			if tm := rcv.HandleLearn(learn); tm != nil {
				teaches = append(teaches, tm)
			}
		}

		for _, teach := range teaches {
			fmt.Printf("Teach from %d: %v\n", teach.sender, teach.announce)
			for id := range nodes[teach.sender].nb {
				if id == teach.sender {
					continue
				}
				rcv := nodes[id]
				if rcv != nil {
					rcv.HandleTeach(teach)
				}
			}
		}
		check()
		switch time {
		case 30:
			delete(nodes, 6)
		case 35:
			//delete(nodes, 5)
		}
		ui(false)
	}
}

func check() {
	for _, p := range nodes {
		if p == nil {
			continue
		}
		fmt.Printf("Node %d: %v\n", p.id, p.ft)

		for _, e := range p.ft {
			if e.peer == p.id {
				fmt.Printf("self forward detected: peer %d, ft: %v, nb: %v\n", p.id, p.ft, p.nb)
				panic("")
			}
			if e.next == p.id {
				fmt.Printf("forward to self detected: peer %d, ft: %v, nb: %v\n", p.id, p.ft, p.nb)
				panic("")
			}
			if e.next != 0 {
				if _, ok := p.nb[e.next]; !ok {
					fmt.Printf("next %d not a neighbor(%v)\n", e.next, p.nb)
					panic("")
				} else {
					t := p.ft[e.next]
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
				n, ok := hop.ft[to]
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
