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
	"flag"
	"fmt"
	"leatea/core"
	"leatea/sim"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// shared variable
var (
	netw   *sim.Network      // Network instance
	redraw bool              // graph modified?
	rt     *sim.RoutingTable // compiled routing table
	graph  *sim.Graph        // reconstructed network graph
	hops   float64           // avg. number of hops in routing table
	routes [][]int           // list of routes
)

// run application
func main() {
	log.Println("LEArn/TEAch routing simulator")
	log.Println("(c) 2022, Bernd Fix   >Y<")

	//------------------------------------------------------------------
	// parse arguments
	var cfgFile string
	flag.StringVar(&cfgFile, "c", "config.json", "JSON-encoded configuration file")
	flag.Parse()

	// read configuration
	if err := sim.ReadConfig(cfgFile); err != nil {
		log.Fatal(err)
	}

	// Build simulation of "physical" environment
	e := sim.BuildEnvironment(sim.Cfg.Env)
	if e == nil {
		log.Fatalf("No environment class '%s' defined.", sim.Cfg.Env.Class)
	}
	// get a canvas for drawing
	c := sim.GetCanvas(sim.Cfg.Render)
	defer c.Close()

	// run simulation depending on canvas mode (dynamic/static)
	if c.IsDynamic() {
		//--------------------------------------------------------------
		// Render to display (update on network change while running)
		//--------------------------------------------------------------

		// start rendering
		if err := c.Open(); err != nil {
			log.Fatal(err)
		}
		// run simulation in go routine to keep main routine
		// available for canvas.
		go run(e)

		// run render loop
		c.Render(func(c sim.Canvas, forced bool) {
			if (forced || redraw) && netw != nil {
				c.Start()
				// render network
				netw.Render(c)
				// render routes (loops)
				for _, route := range routes {
					from := rt.GetNode(route[0])
					for _, hop := range route[1:] {
						to := rt.GetNode(hop)
						c.Line(from.Pos.X, from.Pos.Y, to.Pos.X, to.Pos.Y, 0.3, sim.ClrRed)
						from = to
					}
				}
				redraw = false
			}
		})
	} else {
		//--------------------------------------------------------------
		// Generate final network graph after running the simulation
		//--------------------------------------------------------------

		// run simulation
		run(e)

		// draw final network graph if canvas is not dynamic
		if err := c.Open(); err != nil {
			log.Fatal(err)
		}
		c.Render(func(c sim.Canvas, redraw bool) {
			// render graph
			switch sim.Cfg.Render.Source {
			case "graph":
				graph.Render(c, redraw)
			case "rtab":
				rt.Render(c, redraw)
			default:
				log.Fatal("render: unknown source mode")
			}
			// draw environment
			e.Draw(c)
		})
	}
	log.Println("Done.")
}

func run(e sim.Environment) {
	//------------------------------------------------------------------
	// Build test network
	log.Println("Building network...")
	netw = sim.NewNetwork(e)

	//------------------------------------------------------------------
	// Run test network
	log.Println("Running network...")
	go netw.Run(func(ev *core.Event) {
		// listen to network events
		switch ev.Type {
		case sim.EvNodeAdded:
			log.Printf("[%s] started (#%d)", ev.Peer, ev.Val)
			redraw = true
		case sim.EvNodeRemoved:
			log.Printf("[%s] stopped (%d running)", ev.Peer, ev.Val)
			redraw = true
		case core.EvNeighborExpired:
			log.Printf("[%s] neighbor %s expired", ev.Peer, ev.Ref)
			redraw = true
		case core.EvShorterPath:
			// log.Printf("[%s] short path to %s learned", ev.Peer, ev.Ref)
		case core.EvForwardRemoved:
			// log.Printf("[%s] forward %s removed", ev.Peer, ev.Ref)
		case core.EvLearning:
			// log.Printf("[%s] learning from %s", ev.Peer, ev.Ref)
		case core.EvTeaching:
			// log.Printf("[%s] teaching %s", ev.Peer, ev.Ref)
		}
	})

	//------------------------------------------------------------------
	// prepare monitoring
	sigCh := make(chan os.Signal, 5)
	signal.Notify(sigCh)
	tick := time.NewTicker(time.Second)
	ticks := 0
	epoch := 0
	lastCover := -1.0
	repeat := 1
loop:
	for {
		select {
		case <-tick.C:
			ticks++
			// force redraw
			redraw = true
			// start new epoch?
			if ticks%sim.Cfg.Core.LearnIntv == 0 {
				// start new epoch (every 10 seconds)
				epoch++
				// show status (coverage)
				cover := netw.Coverage()
				log.Printf("Epoch %d: --> Coverage: %.2f%%", epoch, cover)
				rt, graph, hops = netw.RoutingTable()
				if status(rt, nil, hops) > 0 && sim.Cfg.Options.StopOnLoop {
					break loop
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
	//------------------------------------------------------------------
	// stop network
	discarded := netw.Stop()
	log.Printf("Routing complete, %d messages discarded", discarded)

	//------------------------------------------------------------------
	// print final statistics
	trafIn, trafOut := netw.Traffic()
	in := float64(trafIn) / float64(sim.Cfg.Env.NumNodes)
	out := float64(trafOut) / float64(sim.Cfg.Env.NumNodes)
	log.Printf("Avg. traffic per node: %s in / %s out", sim.Scale(in), sim.Scale(out))
	log.Println("Network routing table constructed - checking routes:")
	rt, graph, hops = netw.RoutingTable()
	if status(rt, graph, hops) > 0 {
		// analyze loops
		analyzeLoops(rt)
	}
}

// ----------------------------------------------------------------------
// Follow the route to target. Returns number of hops on success, 0 for
// broken routes and -1 for cycles.
func route(rt [][]int, from, to int) (hops int, route []int) {
	ttl := sim.Cfg.Env.NumNodes
	hops = 0
	for {
		hops++
		route = append(route, from)
		next := rt[from][to]
		if next == to {
			route = append(route, to)
			return
		}
		if next < 0 {
			hops = 0
			return
		}
		from = next
		if ttl--; ttl < 0 {
			route = append(route, to)
			hops = -1
			return
		}
	}
}

// ----------------------------------------------------------------------
// Print status information on routing table (and optional on graph)
// Follow all routes; detect cycles and broken routes
func status(rt *sim.RoutingTable, g *sim.Graph, allHops1 float64) (loops int) {
	success := 0
	broken := 0
	loops = 0
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
			hops, _ := route(rt.List, from, to)
			switch hops {
			case -1:
				loops++
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
	log.Printf("  * Loops: %d (%.2f%%)", loops, perc(loops))
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
	return
}

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
	for from, e := range rt.List {
		for to := range e {
			if from == to {
				continue
			}
			if hops, route := route(rt.List, from, to); hops == -1 {
				// analyze loop
				num := len(route)
				l := &loop{
					from: route[0],
					to:   route[num-1],
				}
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
	for i, l := range loops {
		cl := len(l.cycle)
		found := false
	search:
		for _, e := range routes {
			if cl != len(e) {
				continue
			}
			hop := l.cycle[0]
			for k, hop2 := range e {
				if hop2 == hop {
					// check if rest is the same...
					for q := 0; q < cl; q++ {
						if loops[i].cycle[q] != e[(q+k)%cl] {
							break search
						}
					}
					found = true
					break search
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
	for i, c := range routes {
		buf := new(bytes.Buffer)
		buf.WriteString(fmt.Sprintf("         #%03d: ", i+1))
		for j, id := range c {
			if j > 0 {
				buf.WriteString("-")
			}
			buf.WriteString(rt.GetNode(id).PeerID().String())
		}
		log.Println(buf.String())
	}
	log.Printf("Loop analysis complete.")
}
