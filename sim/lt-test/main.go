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
	"flag"
	"leatea/sim"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	//------------------------------------------------------------------
	// parse arguments
	var mode, env string
	flag.Float64Var(&sim.Width, "w", 100., "width")
	flag.Float64Var(&sim.Length, "l", 100., "length")
	flag.Float64Var(&sim.Reach2, "r", 49., "reach^2")
	flag.Float64Var(&sim.BootupTime, "b", 0, "bootup time")
	flag.IntVar(&sim.NumNodes, "n", 500, "number of nodes")
	flag.StringVar(&mode, "m", "rand", "placement mode")
	flag.StringVar(&env, "e", "open", "environment mode")
	flag.Parse()

	//------------------------------------------------------------------
	// Build and start test network
	log.Println("Building network...")
	p, ok := nodePlacer[mode]
	if !ok {
		log.Fatalf("No topology '%s' defined.", mode)
	}
	e, ok := environment[env]
	if !ok {
		log.Fatalf("No environment '%s' defined.", env)
	}
	netw := sim.NewNetwork(p, e)
	log.Println("Running network...")
	go netw.Run()

	// prepare monitoring
	sigCh := make(chan os.Signal, 5)
	signal.Notify(sigCh)
	tick := time.NewTicker(10 * time.Second)
	lastCover := -1.0
	repeat := 0
loop:
	for {
		select {
		case <-tick.C:
			// show status (coverage)
			cover := netw.Coverage()
			log.Printf("--> Coverage: %.2f%%\n", cover)
			// break loop if coverage has not changed the last 10 epochs
			if lastCover == cover {
				repeat++
				if repeat == 10 {
					break loop
				}
			} else {
				repeat = 0
				lastCover = cover
			}
		case sig := <-sigCh:
			// signal received
			switch sig {
			case syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM:
				break loop
			default:
			}
		}
	}
	// stop network
	discarded := netw.Stop()
	log.Printf("Routing complete, %d messages discarded", discarded)

	// show statistics
	trafIn, trafOut := netw.Traffic()
	in := float64(trafIn) / float64(sim.NumNodes)
	out := float64(trafOut) / float64(sim.NumNodes)
	log.Printf("Avg. traffic per node: %s in / %s out", sim.Scale(in), sim.Scale(out))

	// test routing
	// (1) Follow all routes; detect cycles and broken routes
	success := 0
	broken := 0
	loop := 0
	rt, graph, allHops1 := netw.RoutingTable()
	allHops2 := 0
	allHops3 := 0
	nodes3 := 0
	total := float64(sim.NumNodes * (sim.NumNodes - 1))
	count := 0
	log.Println("Network routing table constructed - checking routes:")
	for from, e := range rt {
		distvec := graph.Distance((from))
		for to := range e {
			if from == to {
				continue
			}
			count++
			hops := route(rt, from, to)
			switch hops {
			case -1:
				loop++
			case 0:
				broken++
			default:
				allHops2 += hops
				success++
			}
			if d := distvec[to]; d != math.MaxInt {
				nodes3++
				allHops3 += d
			}
		}
	}
	perc := func(n int) float64 {
		return float64(100*n) / total
	}
	log.Printf("  * Loops: %d (%.2f%%)\n", loop, perc(loop))
	log.Printf("  * Broken: %d (%.2f%%)\n", broken, perc(broken))
	log.Printf("  * Success: %d (%.2f%%)\n", success, perc(success))
	h2 := float64(allHops2) / float64(success)
	h3 := float64(allHops3) / float64(nodes3)
	log.Printf("  * Hops (routg): %.2f (%d)\n", h2, success)
	log.Printf("  * Hops (table): %.2f\n", allHops1)
	log.Printf("  * Hops (check): %.2f (%d)\n", h3, nodes3)

	// (2) build a graph from the node list and use "shortest path" methods
	//     to compare routes (by number of hops)

	log.Println("Done")
}

// ----------------------------------------------------------------------
// Follow the route to target. Returns number of hops on success, 0 for
// broken routes and -1 for cycles.
func route(rt [][]int, from, to int) int {
	ttl := sim.NumNodes
	hops := 0
	for {
		hops++
		next := rt[from][to]
		if next == to {
			return hops
		}
		if next < 0 {
			return 0
		}
		from = next
		if ttl--; ttl < 0 {
			return -1
		}
	}
}
