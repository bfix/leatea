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
	"encoding/base32"
	"encoding/binary"
	"flag"
	"io"
	"leatea/core"
	"leatea/sim"
	"log"
	"math"
	"os"
	"sort"
)

type LogEntry struct {
	Type     uint32
	TS       int64
	Seq      uint32
	Peer     [32]byte
	Ref      [32]byte
	Target   [32]byte
	WithNext uint32
	NextHop  [32]byte
	Hops     uint32
	TraffIn  uint64
	TraffOut uint64
}

func (e *LogEntry) WithForward() bool {
	return e.Type == core.EvForwardChanged
}

type Forward struct {
	next string
	hops int16
}

type Node struct {
	self     string
	traffIn  uint64
	traffOut uint64
	forwards map[string]*Forward
}

func NewNode(self string) *Node {
	node := new(Node)
	node.self = self
	node.forwards = make(map[string]*Forward)
	return node
}

func (n *Node) SetForward(target, next string, hops int16) {
	forward, ok := n.forwards[target]
	if !ok {
		forward = new(Forward)
		n.forwards[target] = forward
	}
	forward.next = next
	forward.hops = hops
}

var (
	index = make(map[string]*Node)
)

// run application
func main() {
	log.Println("LEArn/TEAch routing analyzer")
	log.Println("(c) 2022, Bernd Fix     >Y<")

	// parse arguments
	var eventLog string
	flag.StringVar(&eventLog, "i", "", "event log (binary)")
	flag.Parse()

	// read event log
	f, err := os.Open(eventLog)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	entries := make([]*LogEntry, 0)
	flag := make([]byte, 1)
	perf := 0
	for k := 1; ; k++ {
		// read and handle next log entry
		ev := new(LogEntry)
		if err := binary.Read(f, binary.BigEndian, &ev.Type); err != nil {
			if err == io.EOF {
				log.Printf("%d log entries read.", k-1)
				break
			}
			log.Fatal(err)
		}
		//log.Printf("type=%d", ev.Type)
		_ = binary.Read(f, binary.BigEndian, &ev.TS)
		_ = binary.Read(f, binary.BigEndian, &ev.Seq)
		_, _ = f.Read(ev.Peer[:])
		self := base32.StdEncoding.EncodeToString(ev.Peer[:5])[:8]
		if _, ok := index[self]; !ok {
			index[self] = NewNode(self)
		}

		switch ev.Type {
		case core.EvForwardChanged, core.EvForwardLearned:
			_, _ = f.Read(ev.Ref[:])
			_, _ = f.Read(ev.Target[:])
			_, _ = f.Read(flag)
			ev.WithNext = 0
			if flag[0] == 1 {
				ev.WithNext = 1
				_, _ = f.Read(ev.NextHop[:])
			}
			var hops int16
			_ = binary.Read(f, binary.BigEndian, &hops)

		case sim.EvNodeTraffic:
			_ = binary.Read(f, binary.BigEndian, &ev.TraffIn)
			_ = binary.Read(f, binary.BigEndian, &ev.TraffOut)
			perf++

		case core.EvNeighborAdded, core.EvNeighborExpired,
			core.EvNeighborUpdated, core.EvRelayRemoved:
			_, _ = f.Read(ev.Ref[:])

		default:
			log.Fatalf("unknown log entry type %d", ev.Type)
		}
		// append to list
		entries = append(entries, ev)
	}
	// sort entries by sequence
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Seq < entries[j].Seq
	})

	// reconstruct forward tables of node step by step
	for _, ev := range entries {
		self := base32.StdEncoding.EncodeToString(ev.Peer[:5])[:8]
		node := index[self]
		ref := base32.StdEncoding.EncodeToString(ev.Ref[:5])[:8]
		switch ev.Type {
		case core.EvForwardChanged, core.EvForwardLearned, core.EvShorterRoute, core.EvRelayRevived, core.EvNeighborRelayed:
			next := ""
			if ev.WithNext == 1 {
				next = base32.StdEncoding.EncodeToString(ev.NextHop[:5])[:8]
			}
			tgt := base32.StdEncoding.EncodeToString(ev.Target[:5])[:8]
			node.SetForward(tgt, next, int16(ev.Hops))

		case sim.EvNodeTraffic:
			node.traffIn = ev.TraffIn
			node.traffOut = ev.TraffOut

		case core.EvNeighborAdded, core.EvNeighborUpdated:
			node.SetForward(ref, "", 0)

		case core.EvNeighborExpired, core.EvRelayRemoved:
			node.SetForward(ref, "", -2)
			delete(index, ref)
		default:
			log.Fatalf("unhandled log entry type %d", ev.Type)
		}
	}

	// traffic statistics
	mIn, mOut := 0., 0.
	dIn, dOut := 0., 0.
	for _, node := range index {
		mIn += float64(node.traffIn)
		mOut += float64(node.traffOut)
	}
	num := float64(perf)
	mIn /= num
	mOut /= num
	for _, node := range index {
		dIn += math.Abs(float64(node.traffIn) - mIn)
		dOut += math.Abs(float64(node.traffOut) - mOut)
	}
	dIn /= num
	dOut /= num
	log.Printf("Traffic (%d peers): %s ±%s in, %s ±%s out",
		perf,
		sim.Scale(mIn), sim.Scale(dIn),
		sim.Scale(mOut), sim.Scale(dOut))

	// run analysis
	analyzeRoutes()
}
