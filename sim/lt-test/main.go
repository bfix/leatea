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
	"leatea/core"
	"leatea/sim"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
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
	tbl    bool              // routing table changed?
)

// run application
func main() {
	log.Println("LEArn/TEAch routing simulator")
	log.Println("(c) 2022, Bernd Fix   >Y<")

	//------------------------------------------------------------------
	// parse arguments
	var cfgFile, profile string
	flag.StringVar(&cfgFile, "c", "config.json", "JSON-encoded configuration file")
	flag.StringVar(&profile, "p", "", "write CPU profile")
	flag.Parse()

	// read configuration
	err := sim.ReadConfig(cfgFile)
	if err != nil {
		log.Fatal(err)
	}
	core.SetConfiguration(sim.Cfg.Core)

	// if we write statistics, create output file
	if len(sim.Cfg.Options.Statistics) > 0 {
		// create file
		if csv, err = os.Create(sim.Cfg.Options.Statistics); err != nil {
			log.Fatal(err)
		}
		defer csv.Close()
		// write header
		_, _ = csv.WriteString("Epoch;Loops;Broken;Success;NumPeers;MeanHops\n")
	}

	// turn on profiling
	if len(profile) > 0 {
		f, err := os.Create(profile)
		if err != nil {
			log.Fatal(err)
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	// Build simulation of "physical" environment
	e := sim.BuildEnvironment(sim.Cfg.Env)
	if e == nil {
		log.Fatalf("No environment class '%s' defined.", sim.Cfg.Env.Class)
	}
	// get a canvas for drawing
	c := sim.GetCanvas(sim.Cfg.Render)
	defer c.Close()

	//------------------------------------------------------------------
	// Build test network
	log.Println("Building network...")
	netw = sim.NewNetwork(e)

	//------------------------------------------------------------------
	// Run test network
	log.Println("Running network...")
	go netw.Run(handleEvent)

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
			if (forced || redraw) && netw.IsActive() {
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
	//------------------------------------------------------------------
	// stop network
	discarded := netw.Stop()
	log.Printf("Routing complete, %d messages discarded", discarded)
	log.Println("Done.")
}

func printEntry(f *core.Entry) string {
	return fmt.Sprintf("{%d,%d,%d,%.3f}",
		netw.GetShortID(f.Peer), netw.GetShortID(f.NextHop),
		f.Hops, f.Origin.Age().Seconds())
}

//nolint:gocyclo // life is complex sometimes...
func handleEvent(ev *core.Event) {
	// check if event is to be displayed.
	show := false
	for _, t := range sim.Cfg.Options.Events {
		if (t < 0 && -t != ev.Type) || (t == ev.Type) {
			show = true
			break
		}
	}
	if !sim.Cfg.Options.ShowEvents {
		show = !show
	}
	// log network events
	switch ev.Type {
	case sim.EvNodeAdded:
		if show {
			val := core.GetVal[[]int](ev)
			log.Printf("[%s] %04X started as #%d (%d running)",
				ev.Peer, ev.Peer.Tag(), val[0], val[1])
		}
		redraw = true
	case sim.EvNodeRemoved:
		val := core.GetVal[[]int](ev)
		remain := val[1]
		if remain < 0 {
			remain = netw.StopNodeByID(ev.Peer)
		}
		if show {
			log.Printf("[%s] #%d stopped (%d running)",
				ev.Peer, val[0], remain)
		}
		redraw = true
		tbl = true
	case core.EvNeighborAdded:
		if show {
			log.Printf("[%d] neighbor #%d added",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
		}
		tbl = true
	case core.EvNeighborUpdated:
		if show {
			log.Printf("[%d] neighbor #%d updated",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
		}
		tbl = true
	case core.EvNeighborExpired:
		if show {
			log.Printf("[%d] neighbor %d expired",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
		}
		redraw = true
		tbl = true
	case core.EvForwardLearned:
		if show {
			e := core.GetVal[*core.Entry](ev)
			log.Printf("[%d < %d] learned %s",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref), printEntry(e))
		}
		tbl = true
	case core.EvForwardChanged:
		if show {
			fw := core.GetVal[[3]*core.Entry](ev)
			log.Printf("[%d < %d] %s < %s > %s",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref),
				printEntry(fw[0]), printEntry(fw[1]), printEntry(fw[2]))
		}
		tbl = true
	case core.EvShorterRoute:
		if show {
			log.Printf("[%d] shorter path to %d learned",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
		}
		tbl = true
	case core.EvRelayRemoved:
		if show {
			log.Printf("[%d] forward to %d removed",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
			tbl = true
		}
		tbl = true
	case core.EvLearning:
		if show {
			log.Printf("[%d] learning from %d",
				netw.GetShortID(ev.Peer), netw.GetShortID(ev.Ref))
		}
	case core.EvTeaching:
		if show {
			val := core.GetVal[[]any](ev)
			msg, _ := val[0].(*core.TEAchMsg)
			counts, _ := val[1].([4]int)
			numAnnounce := len(msg.Announce)
			log.Printf("[%d] teaching: %d removed, %d unfiltered, %d pending, %d skipped",
				netw.GetShortID(ev.Peer), counts[0], counts[1], counts[2], counts[3]-numAnnounce)
			if numAnnounce < 4 {
				announced := make([]string, 0)
				for _, ann := range msg.Announce {
					e := &core.Entry{
						Peer:    ann.Peer,
						Hops:    ann.Hops,
						NextHop: msg.Sender(),
						Origin:  core.TimeFromAge(ann.Age),
					}
					announced = append(announced, printEntry(e))
				}
				log.Printf("[%d] TEAch [%s]",
					netw.GetShortID(ev.Peer), strings.Join(announced, ","))
			}
		}
	case core.EvWantToLearn:
		if show {
			log.Printf("[%d] broadcasting LEArn", netw.GetShortID(ev.Peer))
		}
	}
}

func run(env sim.Environment) {
	//------------------------------------------------------------------
	// prepare monitoring
	sigCh := make(chan os.Signal, 5)
	signal.Notify(sigCh)
	tick := time.NewTicker(time.Second)
	ticks := 0
	epoch := 0
	repeat := 1
	lastFailed := -1
	active := true
	tbl = true
	tblRep := 0
loop:
	for active {
		select {
		case <-tick.C:
			ticks++
			// force redraw
			redraw = true

			// start new epoch?
			if ticks%sim.Cfg.Core.LearnIntv == 0 {
				// start new epoch (every 10 seconds)
				epoch++
				log.Printf("[Epoch %d] %d nodes running", epoch, netw.NumRunning())

				// check routing table changes in the last 10 epochs
				if !tbl {
					tblRep++
				} else {
					tbl = false
					tblRep = 1
				}
				// if no activity, quit simulation.
				if tblRep == 10 {
					log.Printf("Stopped on network inactivity")
					active = false
					return
				}
				// kick off epoch handling go routine.
				go func(epoch int) {
					for _, ev := range env.Epoch(epoch) {
						handleEvent(ev)
					}
					// check if simulation ends
					if sim.Cfg.Options.StopAt > 0 && epoch > sim.Cfg.Options.StopAt {
						log.Printf("Stopped on request")
						active = false
						return
					}

					// show status
					rt, hops = netw.RoutingTable()
					loops, broken, _ := status(epoch, rt, hops)
					if loops > 0 && sim.Cfg.Options.StopOnLoop {
						log.Printf("Stopped on detected loop(s)")
						active = false
						return
					}
					if failed := loops + broken; failed == 0 {
						if lastFailed == failed {
							repeat++
							if repeat == sim.Cfg.Options.MaxRepeat {
								log.Printf("Stopped on repeat limit")
								active = false
								return
							}
						}
						repeat = 1
						lastFailed = -1
					}
				}(epoch)
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
}

// ----------------------------------------------------------------------
// Print status information on routing table (and optional on graph)
// Follow all routes; detect cycles and broken routes
func status(epoch int, rt *sim.RoutingTable, allHops1 float64) (loops, broken, success int) {
	var totalHops int
	loops, broken, success, totalHops = rt.Status()
	num := netw.NumRunning()
	total := loops + broken + success // num * (num - 1)
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
				epoch, loops, broken, success, num, mean)
			_, _ = csv.WriteString(line)
		}
	} else {
		log.Println("  * No routes yet (routing table)")
	}
	return
}
