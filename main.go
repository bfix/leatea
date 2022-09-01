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
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var (
		width, length, reach2 float64
		numNode               int
		withTable             bool
	)
	flag.Float64Var(&width, "w", 100., "width")
	flag.Float64Var(&length, "l", 100., "length")
	flag.Float64Var(&reach2, "r", 49., "reach^2")
	flag.BoolVar(&withTable, "t", false, "table output")
	flag.IntVar(&numNode, "n", 500, "number of nodes")
	flag.Parse()

	log.Println("Building network...")
	netw := sim.NewNetwork(numNode, width, length, reach2)
	log.Println("Running network...")
	go netw.Run()

	sigCh := make(chan os.Signal, 5)
	signal.Notify(sigCh)

	tick := time.NewTicker(10 * time.Second)
loop:
	for {
		select {
		case t := <-tick.C:
			cover := netw.Coverage()
			log.Printf("%s: %.2f%%\n", t.Format(time.RFC1123), cover)
			if cover > 99. {
				break loop
			}
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM:
				break loop
			default:
			}
		}
	}
	netw.Stop()
	log.Println("Routing complete.")
	trafIn, trafOut := netw.Traffic()
	in := float64(trafIn) / float64(numNode)
	out := float64(trafOut) / float64(numNode)
	log.Printf("Avg. traffic per node: %s in / %s out", sim.Scale(in), sim.Scale(out))

	success := 0
	broken := 0
	loop := 0
	rt, allHops1 := netw.FullTable()
	allHops2 := 0
	log.Println("Network routing table constructed - checking routes:")
	for from, e := range rt {
		for to := range e {
			if from == to {
				continue
			}
			ttl := numNode
			hops := 0
			for {
				hops++
				next := rt[from][to]
				if next == to {
					success++
					allHops2 += hops
					break
				}
				if next < 0 {
					broken++
					break
				}
				from = next
				if ttl--; ttl < 0 {
					loop++
					break
				}
			}

		}
	}
	perc := func(n int) float64 {
		return float64(100*n) / float64(numNode*(numNode-1))
	}
	log.Printf("  * Loops: %d (%.2f%%)\n", loop, perc(loop))
	log.Printf("  * Broken: %d (%.2f%%)\n", broken, perc(broken))
	log.Printf("  * Success: %d (%.2f%%)\n", success, perc(success))
	h2 := float64(allHops2) / float64(success)
	log.Printf("  * Hops: %.2f (%.2f)\n", h2, allHops1)

	if withTable {
		log.Println("    1  2  3  4  5  6  7  8  9 10")
		log.Println("  +--+--+--+--+--+--+--+--+--+--+")
		for i, e := range rt {
			s := fmt.Sprintf("%2d|", i+1)
			for _, v := range e {
				s += fmt.Sprintf("%2d|", v)
			}
			log.Println(s)
			log.Println("  +--+--+--+--+--+--+--+--+--+--+")
		}
	}

	log.Println("Done")
}
