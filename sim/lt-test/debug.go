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
	"bytes"
	"fmt"
	"leatea/core"
	"leatea/sim"
	"log"
	"strconv"
)

// ----------------------------------------------------------------------
// Analyze loops
// ----------------------------------------------------------------------

type loop struct {
	from, to int
	head     []int
	cycle    []int
}

func analyzeLoops(rt *sim.RoutingTable) {
	log.Println("Analyzing loops:")
	// collect all loops
	log.Println("  * collect loops:")
	var loops []*loop
	for from, entry := range rt.List {
		for to := range entry.Forwards {
			if from == to {
				continue
			}
			if hops, route := rt.Route(from, to); hops == -1 {
				// analyze loop
				num := len(route)
				l := &loop{from: from, to: to}
			loop:
				for i, hop := range route {
					for j := i + 1; j < num; j++ {
						if hop == route[j] {
							l.head = route[:i]
							l.cycle = route[i:j]
							loops = append(loops, l)
							break loop
						}
					}
				}
			}
		}
	}
	log.Printf("      -> %d loops found.", len(loops))

	// check for distinct cycles
	log.Println("  * find distinct loops:")
	routes = make([][]int, 0)
	for _, l := range loops {
		cl := len(l.cycle)
		found := false
	search:
		for _, e := range routes {
			if cl == len(e) {
				hop := l.cycle[0]
				for k, hop2 := range e {
					if hop2 == hop {
						// check if rest is the same...
						for q := 1; q < cl; q++ {
							r := (q + k) % cl
							if l.cycle[q] != e[r] {
								continue search
							}
						}
						found = true
						break search
					}
				}
			}
		}
		if !found {
			routes = append(routes, l.cycle)
		}
	}
	redraw = true
	log.Printf("      -> %d distinct loops found:", len(routes))

	// show distinct cycles
	cv := func(p *core.PeerID) string {
		if p == nil {
			return "0"
		}
		return strconv.Itoa(rt.Index[p.Key()])
	}
	rogues := make(map[int]int)
	for i, c := range routes {
		buf := new(bytes.Buffer)
		buf.WriteString(fmt.Sprintf("         #%03d: ", i+1))
		for j, id := range c {
			if j > 0 {
				buf.WriteString("-")
			}
			node := rt.List[id].Node
			nid := netw.GetShortID(node.PeerID())
			buf.WriteString(strconv.Itoa(nid))
			count, ok := rogues[nid]
			if !ok {
				count = 0
			}
			rogues[nid] = count + 1
		}
		log.Println(buf.String())
	}
	// Dump forward tables of impacted nodes
	for nid, count := range rogues {
		log.Printf("Peer #%d (%d times)", nid, count)
		node := rt.List[nid].Node
		log.Printf("  Tbl = %s", node.ListTable(cv))
	}
	log.Printf("Loop analysis complete.")
}
