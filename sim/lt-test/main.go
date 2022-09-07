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
	"fmt"
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
	var cfgFile string
	flag.StringVar(&cfgFile, "c", "config.json", "JSON-encoded configuration file")
	flag.Parse()

	// read configuration
	if err := sim.ReadConfig(cfgFile); err != nil {
		log.Fatal(err)
	}

	//------------------------------------------------------------------
	// Build and start test network
	log.Println("Building network...")
	e := getEnvironment(sim.Cfg.Env)
	if e == nil {
		log.Fatalf("No environment class '%s' defined.", sim.Cfg.Env.Class)
	}
	netw := sim.NewNetwork(e)
	log.Println("Running network...")
	go netw.Run()

	// prepare monitoring
	sigCh := make(chan os.Signal, 5)
	signal.Notify(sigCh)
	tick := time.NewTicker(10 * time.Second)
	epoch := 0
	lastCover := -1.0
	repeat := 1
loop:
	for {
		select {
		case <-tick.C:
			epoch++
			// show status (coverage)
			cover := netw.Coverage()
			log.Printf("--> Coverage: %.2f%%", cover)
			rt, _, hops := netw.RoutingTable()
			status(rt, nil, hops)

			// generate SVG if "video" mode is set.
			if sim.Cfg.Options.Video {
				f, err := os.Create(fmt.Sprintf("%s.%03d.svg", sim.Cfg.Options.SVGFile, epoch))
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()
				rt.SVG(f)

			}
			// if all nodes are running break loop if coverage has not
			// changed for some epochs (if defined)
			if !netw.Booted() || sim.Cfg.Options.MaxRepeat == 0 {
				continue
			}
			if lastCover == cover {
				repeat++
				if repeat == sim.Cfg.Options.MaxRepeat {
					break loop
				}
			} else {
				repeat = 1
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

	// print final statistics
	trafIn, trafOut := netw.Traffic()
	in := float64(trafIn) / float64(sim.Cfg.Env.NumNodes)
	out := float64(trafOut) / float64(sim.Cfg.Env.NumNodes)
	log.Printf("Avg. traffic per node: %s in / %s out", sim.Scale(in), sim.Scale(out))
	log.Println("Network routing table constructed - checking routes:")
	rt, graph, hops := netw.RoutingTable()
	status(rt, graph, hops)

	// build SVG on demand
	if len(sim.Cfg.Options.SVGFile) > 0 {
		f, err := os.Create(sim.Cfg.Options.SVGFile + ".svg")
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		switch sim.Cfg.Options.SVGMode {
		case "graph":
			graph.SVG(f)
		case "rt":
			rt.SVG(f)
		default:
			log.Fatal("unknown SVG mode")
		}
	}
	log.Println("Done")
}

// ----------------------------------------------------------------------
// Follow the route to target. Returns number of hops on success, 0 for
// broken routes and -1 for cycles.
func route(rt [][]int, from, to int) int {
	ttl := sim.Cfg.Env.NumNodes
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

// Print status information on routing table (and optional on graph)
// Follow all routes; detect cycles and broken routes
func status(rt *sim.RoutingTable, g *sim.Graph, allHops1 float64) {
	success := 0
	broken := 0
	loop := 0
	allHops2 := 0
	allHops3 := 0
	nodes3 := 0
	total := float64(sim.Cfg.Env.NumNodes * (sim.Cfg.Env.NumNodes - 1))
	count := 0
	var distvec []int
	for from, e := range rt.List {
		if g != nil {
			distvec = g.Distance((from))
		}
		for to := range e {
			if from == to {
				continue
			}
			count++
			hops := route(rt.List, from, to)
			switch hops {
			case -1:
				loop++
			case 0:
				broken++
			default:
				allHops2 += hops
				success++
			}
			if g != nil {
				if d := distvec[to]; d != math.MaxInt {
					nodes3++
					allHops3 += d
				}
			}
		}
	}
	perc := func(n int) float64 {
		return float64(100*n) / total
	}
	log.Printf("  * Loops: %d (%.2f%%)", loop, perc(loop))
	log.Printf("  * Broken: %d (%.2f%%)", broken, perc(broken))
	log.Printf("  * Success: %d (%.2f%%)", success, perc(success))
	h2 := float64(allHops2) / float64(success)
	h3 := float64(allHops3) / float64(nodes3)
	if success > 0 {
		log.Printf("  * Hops (routg): %.2f (%d)", h2, success)
		log.Printf("  * Hops (table): %.2f", allHops1)
	}
	if g != nil {
		log.Printf("  * Hops (graph): %.2f (%d)", h3, nodes3)
	}
}
