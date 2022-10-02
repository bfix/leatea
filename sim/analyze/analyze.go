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
	"log"
	"sort"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// Analyze routes for loops and broken routes
// ----------------------------------------------------------------------

func route(fromNode, toNode *Node) (hops int, route []string) {
	ttl := len(nodes)
	hops = 0
	from := fromNode.self
	to := toNode.self
	for {
		route = append(route, from)
		hops++
		forward, ok := fromNode.forwards[to]
		if !ok {
			hops = 0
			return
		}
		if forward.next == "" {
			route = append(route, to)
			return
		}
		if forward.hops < 0 {
			hops = 0
			return
		}
		from = forward.next
		fromNode = nodes[from]
		if ttl--; ttl < 0 {
			hops = -1
			return
		}
	}
}

type Loop struct {
	from, to string
	head     []string
	cycle    []string
}

type Result struct {
	loops     int
	broken    int
	success   int
	totalHops int
	bestTo    *Node
	bestFrom  *Node
	bestHops  int
	bestRoute []string
	loopList  []*Loop
	probs     map[string]int
}

func analyzeRoutes() (res *Result) {
	res = new(Result)
	res.probs = make(map[string]int)

	// find longest broken route
	for _, from := range nodes {
		for _, to := range nodes {
			if from.self == to.self {
				continue
			}
			hops, route := route(from, to)
			if hops == -1 {
				res.loops++
				// analyze loop
				num := len(route)
				l := &Loop{from: from.self, to: to.self}
			loop:
				for i, hop := range route {
					for j := i + 1; j < num; j++ {
						if hop == route[j] {
							l.head = route[:i]
							l.cycle = route[i:j]
							res.loopList = append(res.loopList, l)
							break loop
						}
					}
				}
			} else if hops == 0 {
				res.broken++
				idx := route[len(route)-1]
				v := res.probs[idx]
				res.probs[idx] = v + 1
				if len(route) > res.bestHops {
					res.bestHops = len(route)
					res.bestRoute = route
					res.bestFrom = from
					res.bestTo = to
				}
			} else {
				res.totalHops += hops
				res.success++
			}
		}
	}
	return
}

func analyzeLoops(res *Result) {
	// check for cycles
	if res.loops > 0 {
		log.Printf("      -> %d loops found.", res.loops)
		log.Println("  * finding distinct loops:")
		routes := make([][]string, 0)
		for _, l := range res.loopList {
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
		rogues := make(map[string]int)
		for i, c := range routes {
			buf := new(bytes.Buffer)
			buf.WriteString(fmt.Sprintf("         #%03d: ", i+1))
			for j, id := range c {
				if j > 0 {
					buf.WriteString("-")
				}
				buf.WriteString(id)
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
			log.Printf("  Peer #%s (%d times)", id, count)
			log.Printf("    Tbl = %s", listForwards(id))
		}
		log.Printf("  Loop analysis complete.")
	}
}

func analyzeBroken(res *Result) {
	if res.broken > 0 {
		log.Printf("      -> %d routes are broken:", res.broken)
		for idx, count := range res.probs {
			node := nodes[idx]
			log.Printf("    %s (%d): %d entries", idx, count, len(node.forwards))
		}

		// show route
		log.Printf("  Broken route %s -> %s: %v", res.bestFrom.self, res.bestTo.self, res.bestRoute)
		last := res.bestRoute[len(res.bestRoute)-1]
		node := nodes[last]
		log.Printf("  Break at %s: Tbl = %s", last, listForwards(last))
		log.Println("  Neighbors:")
		for tgt, entry := range node.forwards {
			if entry.next == "" {
				log.Printf("    %s: Tbl = %s", tgt, listForwards(tgt))
			}
		}
		log.Printf("  Target %s: Tbl = %s", res.bestTo.self, listForwards(res.bestTo.self))
		log.Printf("  Broken route analysis complete.")
	}
	log.Printf("Route analysis complete:")
}

func listForwards(id string) string {
	node := nodes[id]
	entries := make([]string, 0)
	for tgt, e := range node.forwards {
		s := fmt.Sprintf("{%s,%s,%d}", tgt, e.next, e.hops)
		entries = append(entries, s)
	}
	sort.Slice(entries, func(i, j int) bool {
		s1, _ := strconv.Atoi(entries[i][1:strings.Index(entries[i], ",")])
		s2, _ := strconv.Atoi(entries[j][1:strings.Index(entries[j], ",")])
		return s1 < s2
	})
	return "[" + strings.Join(entries, ",") + "]"
}
