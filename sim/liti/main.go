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
	"context"
	"flag"
	"fmt"
	"leatea/core"
	"leatea/sim"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"
)

// shared variable
var (
	ctx     context.Context    // execution context
	cancel  context.CancelFunc // kill switch
	netw    *sim.Network       // Network instance
	changed bool               // routing modified?
	redraw  bool               // graph modified?
	rt      *sim.RoutingTable  // compiled routing table
	routes  [][]int            // list of routes
	csv     *os.File           // statistics output
	evHdlr  *EventHandler      // event handler
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
		_, _ = csv.WriteString("Epoch,Loops,Broken,Success,NumPeers,Started,StopPending,MeanHops\n")
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
	if c != nil {
		defer c.Close()
	}

	//------------------------------------------------------------------
	// Build test network
	log.Println("Building network...")
	netw = sim.NewNetwork(e, sim.Cfg.Env.NumNodes)

	//------------------------------------------------------------------
	// Create event handler
	evHdlr = NewEventHandler(netw.GetShortID)

	//------------------------------------------------------------------
	// create base context
	ctx, cancel = context.WithCancel(context.Background())

	//------------------------------------------------------------------
	// Run test network
	log.Println("Running network...")
	go netw.Run(ctx, evHdlr.HandleEvent)

	// run simulation depending on canvas mode (dynamic/static)
	if sim.Cfg.Render.Dynamic && c != nil && c.IsDynamic() {
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

		if c != nil {
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
	}
	//------------------------------------------------------------------
	// stop network
	discarded := netw.Stop()
	log.Printf("Routing complete, %d messages discarded", discarded)
	log.Println("Done.")
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
	unchangedCount := 1

	// as long as active...
loop:
	for {
		select {
		case <-ctx.Done():
			cancel()
			break loop
		case <-tick.C:
			ticks++
			// force redraw
			redraw = true

			// start new epoch?
			if ticks%sim.Cfg.Core.LearnIntv == 0 {
				// start new epoch (every 10 seconds)
				epoch++

				// check routing table changes in the last epoch
				if changed, redraw = evHdlr.State(); !changed {
					unchangedCount++
				} else {
					unchangedCount = 1
				}
				// if no activity on a settled network within 3 epochs, quit simulation.
				if netw.Settled() && unchangedCount > 3 {
					log.Printf("Stopped on network inactivity")
					cancel()
					continue
				}
				running, started, removals := netw.Stats()
				log.Printf("[Epoch %d] %d nodes running (%d started, %d removals pending, %d settled)",
					epoch, running, started, removals, unchangedCount)

				// handle events generated by the environment
				for _, ev := range env.Epoch(epoch) {
					if ev.Type == sim.EvNodeRemoved {
						val := core.GetVal[[]int](ev)
						if val[1] < 0 {
							netw.StopNodeByID(ev.Peer)
						}
					}
				}
				// kick off epoch handling go routine.
				go func(epoch int) {
					log.Printf("Handling epoch tasks...")

					// check if simulation ends
					if sim.Cfg.Options.StopAt > 0 && epoch > sim.Cfg.Options.StopAt {
						log.Printf("Stopped on request")
						cancel()
						return
					}
					// show status
					if sim.Cfg.Options.EpochStatus {
						rt = netw.RoutingTable()
						loops, broken, _ := status(epoch, rt)
						if loops > 0 && sim.Cfg.Options.StopOnLoop {
							log.Printf("Stopped on detected loop(s)")
							cancel()
							return
						}
						if failed := loops + broken; failed == 0 {
							if lastFailed == failed {
								repeat++
								if repeat == sim.Cfg.Options.MaxRepeat {
									log.Printf("Stopped on repeat limit")
									cancel()
									return
								}
							}
							repeat = 1
							lastFailed = -1
						}
					}
				}(epoch)
			}
		case sig := <-sigCh:
			// signal received
			switch sig {
			case syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM:
				cancel()
			default:
			}
		}
	}

	//------------------------------------------------------------------
	// print final statistics
	log.Println("Network routing table constructed - checking routes:")
	rt = netw.RoutingTable()
	status(epoch, rt)

	// dump routing on demand
	if len(sim.Cfg.Options.TableDump) > 0 {
		netw.DumpRouting(sim.Cfg.Options.TableDump)
	}
}

// ----------------------------------------------------------------------
// Print status information on routing table (and optional on graph)
// Follow all routes; detect cycles and broken routes
func status(epoch int, rt *sim.RoutingTable) (loops, broken, success int) {
	var totalHops int
	loops, broken, success, totalHops = rt.Status()
	num, started, stopPending := netw.Stats()
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
			line := fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%.2f\n",
				epoch, loops, broken, success, num, started, stopPending, mean)
			_, _ = csv.WriteString(line)
		}
	} else {
		log.Println("  * No routes yet (routing table)")
	}
	return
}
