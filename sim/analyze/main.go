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
	"os"

	"github.com/bfix/gospel/data"
)

var (
	dump  *sim.Dump
	index = make(map[int]*sim.DumpNode)
)

// run application
func main() {
	log.Println("LEArn/TEAch routing analyzer")
	log.Println("(c) 2022, Bernd Fix     >Y<")

	// parse arguments
	var dumpFile string
	flag.StringVar(&dumpFile, "i", "", "routing table dump (binary)")
	flag.Parse()

	// read dump
	f, err := os.Open(dumpFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fi, _ := f.Stat()
	dump = new(sim.Dump)
	if err := data.UnmarshalStream(f, &dump, int(fi.Size())); err != nil {
		log.Fatal(err)
	}
	log.Printf("Number of nodes: %d", dump.NumNodes)

	for _, node := range dump.Nodes {
		index[int(node.ID)] = node
	}

	// run analysis
	analyzeLoops()
	analyzeBroken()
}
