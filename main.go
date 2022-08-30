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
	"time"
)

func main() {
	var (
		width, length, reach2 float64
		numNode               int
	)
	flag.Float64Var(&width, "w", 100., "width")
	flag.Float64Var(&length, "l", 100., "length")
	flag.Float64Var(&reach2, "r", 49., "reach^2")
	flag.IntVar(&numNode, "n", 500, "number of nodes")
	flag.Parse()

	log.Println("Building network...")
	netw := sim.NewNetwork(numNode, width, length, reach2)
	log.Println("Running network...")
	go netw.Run()

	tick := time.NewTicker(30 * time.Second)
	for {
		t := <-tick.C
		log.Printf("%s: %f%%\n", t.String(), netw.Coverage())
	}
}
