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
	"leatea/sim"
	"log"
	"sort"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// Analyze loops
// ----------------------------------------------------------------------

type loop struct {
	from, to int
	head     []int
	cycle    []int
}

func analyzeLoops() {
	log.Println("Analyzing loops:")
	// collect all loops
	log.Println("  * collect loops:")
	var loops []*loop
	for _, from := range dump.Nodes {
		for _, to := range dump.Nodes {
			if from.ID == to.ID {
				continue
			}
			if hops, route := route(from, to); hops == -1 {
				// analyze loop
				num := len(route)
				l := &loop{from: int(from.ID), to: int(to.ID)}
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
	if len(loops) > 0 {
		log.Println("  * find distinct loops:")
		routes := make([][]int, 0)
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
		log.Printf("      -> %d distinct loops found:", len(routes))

		// show distinct cycles
		rogues := make(map[int]int)
		for i, c := range routes {
			buf := new(bytes.Buffer)
			buf.WriteString(fmt.Sprintf("         #%03d: ", i+1))
			for j, id := range c {
				if j > 0 {
					buf.WriteString("-")
				}
				buf.WriteString(strconv.Itoa(id))
				count, ok := rogues[id]
				if !ok {
					count = 0
				}
				rogues[id] = count + 1
			}
			log.Println(buf.String())
		}
		// Dump forward tables of impacted nodes
		for id, count := range rogues {
			log.Printf("Peer #%d (%d times)", id, count)
			log.Printf("  Tbl = %s", listForwards(id))
		}
	}
	log.Printf("Loop analysis complete.")
}

func route(fromNode, toNode *sim.DumpNode) (hops int, route []int) {
	ttl := int(dump.NumNodes)
	hops = 0
	from := int(fromNode.ID)
	to := int(toNode.ID)
	for {
		route = append(route, from)
		hops++
		next := 0
		for _, entry := range fromNode.Tbl {
			if int(entry.Peer) == to {
				if entry.Next == 0 {
					route = append(route, to)
					return
				}
				if entry.Hops < 0 {
					hops = 0
					return
				}
				next = int(entry.Next)
				break
			}
		}
		if next == 0 {
			hops = 0
			return
		}
		from = next
		fromNode = index[from]
		if ttl--; ttl < 0 {
			hops = -1
			return
		}
	}
}

func listForwards(id int) string {
	node := index[id]
	entries := make([]string, 0)
	for _, e := range node.Tbl {
		s := fmt.Sprintf("{%d,%d,%d,%.3f}", e.Peer, e.Next, e.Hops, e.Age())
		entries = append(entries, s)
	}
	sort.Slice(entries, func(i, j int) bool {
		s1, _ := strconv.Atoi(entries[i][1:strings.Index(entries[i], ",")])
		s2, _ := strconv.Atoi(entries[j][1:strings.Index(entries[j], ",")])
		return s1 < s2
	})
	return "[" + strings.Join(entries, ",") + "]"
}

func analyzeBroken() {
	log.Println("Analyzing broken route:")

	// find longest broken route
	var (
		bestTo, bestFrom *sim.DumpNode
		bestHops         = 0
		bestRoute        []int
	)
	log.Println("  * inspect longest broken route:")
	for _, from := range dump.Nodes {
		if !from.Running {
			continue
		}
		for _, to := range dump.Nodes {
			if !to.Running || from.ID == to.ID {
				continue
			}
			hops, route := route(from, to)
			//log.Printf("%d -> %d: %v", from.ID, to.ID, route)
			if hops == 0 {
				if len(route) > bestHops {
					bestHops = len(route)
					bestRoute = route
					bestFrom = from
					bestTo = to
				}
			}
		}
	}
	// show route
	log.Printf("Broken route %d -> %d: %v", bestFrom.ID, bestTo.ID, bestRoute)
	last := bestRoute[len(bestRoute)-1]
	node := index[last]
	log.Printf("Break at %d: Tbl = %s", last, listForwards(last))
	log.Println("Neighbors:")
	for _, entry := range node.Tbl {
		if entry.Next == 0 {
			log.Printf("  %d: Tbl = %s", entry.Peer, listForwards(int(entry.Peer)))
		}
	}
	log.Printf("Target %d: Tbl = %s", bestTo.ID, listForwards(int(bestTo.ID)))

	log.Printf("Broken route analysis complete.")
}
