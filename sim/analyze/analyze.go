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

type loop struct {
	from, to string
	head     []string
	cycle    []string
}

func analyzeRoutes() {
	log.Printf("Analyzing routes between %d peers:", len(index))

	// find longest broken route
	var (
		bestTo, bestFrom *Node
		bestHops         = 0
		bestRoute        []string
		loopList         []*loop
	)
	broken := 0
	loops := 0
	success := 0
	totalHops := 0
	probs := make(map[string]int)

	for _, from := range index {
		for _, to := range index {
			if from.self == to.self {
				continue
			}
			hops, route := route(from, to)
			if hops == -1 {
				loops++
				// analyze loop
				num := len(route)
				l := &loop{from: from.self, to: to.self}
			loop:
				for i, hop := range route {
					for j := i + 1; j < num; j++ {
						if hop == route[j] {
							l.head = route[:i]
							l.cycle = route[i:j]
							loopList = append(loopList, l)
							break loop
						}
					}
				}
			} else if hops == 0 {
				broken++
				idx := route[len(route)-1]
				v := probs[idx]
				probs[idx] = v + 1
				if len(route) > bestHops {
					bestHops = len(route)
					bestRoute = route
					bestFrom = from
					bestTo = to
				}
			} else {
				totalHops += hops
				success++
			}
		}
	}

	// analze loops

	log.Printf("      -> %d loops found.", loops)

	// check for distinct cycles
	if loops > 0 {
		log.Println("  * finding distinct loops:")
		routes := make([][]string, 0)
		for _, l := range loopList {
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

	log.Printf("      -> %d routes are broken:", broken)
	if broken > 0 {
		for idx, count := range probs {
			node := index[idx]
			log.Printf("    %s (%d): %d entries", idx, count, len(node.forwards))
		}

		// show route
		log.Printf("  Broken route %s -> %s: %v", bestFrom.self, bestTo.self, bestRoute)
		last := bestRoute[len(bestRoute)-1]
		node := index[last]
		log.Printf("  Break at %s: Tbl = %s", last, listForwards(last))
		log.Println("  Neighbors:")
		for tgt, entry := range node.forwards {
			if entry.next == "" {
				log.Printf("    %s: Tbl = %s", tgt, listForwards(tgt))
			}
		}
		log.Printf("  Target %s: Tbl = %s", bestTo.self, listForwards(bestTo.self))
		log.Printf("  Broken route analysis complete.")
	}
	log.Printf("Route analysis complete:")

	num := len(index)
	total := num * (num - 1)
	if total > 0 {
		perc := func(n int) float64 {
			return float64(100*n) / float64(total)
		}
		log.Printf("  * Loops: %d (%.2f%%)", loops, perc(loops))
		log.Printf("  * Broken: %d (%.2f%%)", broken, perc(broken))
		log.Printf("  * Success: %d (%.2f%%)", success, perc(success))
		if success > 0 {
			mean := float64(totalHops) / float64(success)
			log.Printf("  * Hops (routg): %.2f (%d)", mean, success)
		}
	}
}

func route(fromNode, toNode *Node) (hops int, route []string) {
	ttl := len(index)
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
		fromNode = index[from]
		if ttl--; ttl < 0 {
			hops = -1
			return
		}
	}
}

func listForwards(id string) string {
	node := index[id]
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
