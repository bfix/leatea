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
	"log"
)

var (
	width    = 100. // width of world (unit)
	length   = 100. // length of world (unit)
	reach2   = 49.  // max distance^2 for contact (unit^2)
	numNode  = 500  // number of nodes
	epoch    = 30   // learn interval (in epochs)
	steps    = 1000 // number of iterations
	maxTeach = 5    // max number of entries in teach
)

func main() {
	flag.Float64Var(&width, "w", 100., "width")
	flag.Float64Var(&length, "l", 100., "length")
	flag.Float64Var(&reach2, "r", 49., "reach^2")
	flag.IntVar(&numNode, "n", 500, "number of nodes")
	flag.IntVar(&epoch, "e", 30, "learn interval (in epochs)")
	flag.IntVar(&steps, "i", 1000, "iterations")
	flag.Parse()
	netw := NewNetwork()
	netw.Run(steps, log1)
}

func log1(epoch, pending, learned, total, responses int) {
	log.Printf("Epoch #%d\n", epoch)
	log.Printf("  Pending messages: %d\n", pending)
	log.Printf("  Learned peers: %d\n", learned)
	know := float64(100*total) / float64((numNode-1)*(numNode-1))
	log.Printf("  Knowledge: %.2f%%\n", know)
	log.Printf("  Responses: %d\n", responses)
}

func log2(epoch, pending, learned, total, responses int) {
	know := float64(100*total) / float64((numNode-1)*(numNode-1))
	fmt.Printf("%d;%f\n", epoch, know)
}
