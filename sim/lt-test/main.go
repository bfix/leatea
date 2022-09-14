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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// shared variable
var (
	netw   *sim.Network      // Network instance
	redraw bool              // graph modified?
	rt     *sim.RoutingTable // compiled routing table
	hops   float64           // avg. number of hops in routing table
	routes [][]int           // list of routes
	csv    *os.File          // statistics output
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
	err := sim.ReadConfig(cfgFile)
	if err != nil {
		log.Fatal(err)
	}

	// if we write statistics, create output file
	if len(sim.Cfg.Options.Statistics) > 0 {
		// create file
		if csv, err = os.Create(sim.Cfg.Options.Statistics); err != nil {
			log.Fatal(err)
		}
		defer csv.Close()
		// write header
		_, _ = csv.WriteString("Epoch;Loops;Broken;Success;Total;MeanHops\n")
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
				// render routes
				for _, route := range routes {
					from := rt.List[route[0]].Node
					for _, hop := range route[1:] {
						to := rt.List[hop].Node
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
			c.Start()
			// render routing table
			rt.Render(c)
			// draw environment
			e.Draw(c)
		})
	}
	log.Println("Done.")
}

func printEntry(f *core.Entry) string {
	return fmt.Sprintf("{%d,%d,%d,%.2f}",
		netw.GetShortID(f.Peer), f.Hops,
		netw.GetShortID(f.NextHop), f.Origin.Age().Seconds())
}

func handleEvent(ev *core.Event) {
	// check if event is to be displayed.
	show := false
	for _, t := range sim.Cfg.Options.ShowEvents {
		if t == ev.Type {
			show = true
			break
		}
	}
	if !show {
		return
	}
	// log network events
	switch ev.Type {
	case sim.EvNodeAdded:
		val := core.GetVal[[]int](ev)
		log.Printf("[%s] started as #%d (%d running)",
			ev.Peer, val[0], val[1])
		redraw = true
	case sim.EvNodeRemoved:
		val := core.GetVal[[]int](ev)
		remain := val[1]
		if remain < 0 {
			remain = netw.StopNodeByID(ev.Peer)
		}
		log.Printf("[%s] #%d stopped (%d running)",
			ev.Peer, val[0], remain)
		redraw = true
	case core.EvNeighborAdded:
		log.Printf("[%d] neighbor #%d added",
			netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
	case core.EvNeighborUpdated:
		log.Printf("[%d] neighbor #%d updated",
			netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
	case core.EvNeighborExpired:
		log.Printf("[%d] neighbor %s expired",
			netw.GetShortID(ev.Peer), ev.Ref)
		redraw = true
	case core.EvForwardLearned:
		e := core.GetVal[*core.Entry](ev)
		log.Printf("[%d < %d] learned %s",
			netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref), printEntry(e))
	case core.EvForwardChanged:
		fw := core.GetVal[[3]*core.Entry](ev)
		log.Printf("[%d < %d] %s < %s > %s",
			netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref),
			printEntry(fw[0]), printEntry(fw[1]), printEntry(fw[2]))
	case core.EvShorterPath:
		log.Printf("[%d] shorter path to %d learned",
			netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
	case core.EvForwardRemoved:
		log.Printf("[%d] forward to %d removed",
			netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
	case core.EvLearning:
		log.Printf("[%d] learning from %d",
			netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
	case core.EvTeaching:
		msg := core.GetVal[*core.TEAchMsg](ev)
		announced := make([]string, 0)
		for _, ann := range msg.Announce {
			e := &core.Entry{
				Forward: *ann,
				NextHop: msg.Sender(),
				Origin:  core.TimeFromAge(ann.Age),
			}
			announced = append(announced, printEntry(e))
		}
		log.Printf("[%d] teaching [%s]",
			netw.GetShortID(ev.Peer), strings.Join(announced, ","))
	case core.EvWantToLearn:
		log.Printf("[%d] broadcasting LEArn", netw.GetShortID(ev.Peer))
	}
}

func run(env sim.Environment) {
	//------------------------------------------------------------------
	// Build test network
	log.Println("Building network...")
	netw = sim.NewNetwork(env)

	//------------------------------------------------------------------
	// Run test network
	log.Println("Running network...")
	go netw.Run(handleEvent)

	//------------------------------------------------------------------
	// prepare monitoring
	sigCh := make(chan os.Signal, 5)
	signal.Notify(sigCh)
	tick := time.NewTicker(time.Second)
	ticks := 0
	epoch := 0
	repeat := 1
	lastFailed := -1
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
				log.Printf("[Epoch %d]", epoch)
				for _, ev := range env.Epoch(epoch) {
					handleEvent(ev)
				}
				// check if simulation ends
				if sim.Cfg.Options.StopAt > 0 && epoch > sim.Cfg.Options.StopAt {
					log.Printf("Stopped on request")
					break loop
				}

				// show status
				rt, hops = netw.RoutingTable()
				loops, broken, _ := status(epoch, rt, hops)
				if loops > 0 && sim.Cfg.Options.StopOnLoop {
					log.Printf("Stopped on detected loop(s)")
					break loop
				}
				if failed := loops + broken; failed == 0 {
					if lastFailed == failed {
						repeat++
						if repeat == sim.Cfg.Options.MaxRepeat {
							log.Printf("Stopped on repeat limit")
							break loop
						}
					}
					repeat = 1
					lastFailed = -1
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
	// print final statistics
	trafIn, trafOut := netw.Traffic()
	in := float64(trafIn) / float64(sim.Cfg.Env.NumNodes)
	out := float64(trafOut) / float64(sim.Cfg.Env.NumNodes)
	log.Printf("Avg. traffic per node: %s in / %s out", sim.Scale(in), sim.Scale(out))
	log.Println("Network routing table constructed - checking routes:")
	rt, hops = netw.RoutingTable()
	loops, _, _ := status(epoch, rt, hops)
	if loops > 0 {
		analyzeLoops(rt)
	}
	//------------------------------------------------------------------
	// stop network
	discarded := netw.Stop()
	log.Printf("Routing complete, %d messages discarded", discarded)
}

// ----------------------------------------------------------------------
// Print status information on routing table (and optional on graph)
// Follow all routes; detect cycles and broken routes
func status(epoch int, rt *sim.RoutingTable, allHops1 float64) (loops, broken, success int) {
	var totalHops int
	loops, broken, success, totalHops = rt.Status()
	num := netw.NumRunning()
	total := num * (num - 1)
	if total > 0 {
		// log statistics to console
		perc := func(n int) float64 {
			return float64(100*n) / float64(total)
		}
		log.Printf("  * Loops: %d (%.2f%%)", loops, perc(loops))
		log.Printf("  * Broken: %d (%.2f%%)", broken, perc(broken))
		log.Printf("  * Success: %d (%.2f%%)", success, perc(success))
		mean := 0.
		if success > 0 {
			mean = float64(totalHops) / float64(success)
			log.Printf("  * Hops (routg): %.2f (%d)", mean, success)
		}
		// log statistics to file if requested
		if csv != nil {
			line := fmt.Sprintf("%d,%d,%d,%d,%d,%.2f\n",
				epoch, loops, broken, success, total, mean)
			_, _ = csv.WriteString(line)
		}
	} else {
		log.Println("  * No routes yet (routing table)")
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
	for from, entry := range rt.List {
		for _, to := range entry.Forwards {
			if from == to {
				continue
			}
			if hops, route := rt.Route(from, to); hops == -1 {
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
			buf.WriteString(rt.List[id].Node.PeerID().String())
		}
		log.Println(buf.String())
	}
	log.Printf("Loop analysis complete.")
}
