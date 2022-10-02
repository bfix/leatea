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
	"fmt"
	"io"
	"leatea/core"
	"leatea/sim"
	"log"
	"math"
	"os"
	"sort"
)

// LogEntry is a representation of an entry in the log file
type LogEntry struct {
	// mandatory fields
	Type uint32   // event type
	TS   int64    // time stamp (event handler)
	Seq  uint32   // sequence number (global)
	Peer [32]byte // event sender

	// EvForwardChanged, EvForwardLearned, EvNeighborAdded,
	// EvNeighborExpired, EvNeighborUpdated, EvRelayRemoved
	// EvTraffic
	Ref [32]byte // reference peer

	// EvForwardChanged, EvForwardLearned
	Target   [32]byte
	WithNext uint32
	NextHop  [32]byte
	Hops     uint32

	// EvNodeTraffic
	TraffIn  uint64
	TraffOut uint64

	// EvNodeAdded
	Running  uint16
	Pending  uint16
	X, Y, R2 float64
}

// Forward in simplified form (no timing information)
type Forward struct {
	next string
	hops int16
}

// Node in the ad-hoc network; reconstructed from log events
type Node struct {
	self     string
	traffIn  uint64
	traffOut uint64
	forwards map[string]*Forward
	idx      int
	x, y, r2 float64
}

// NewNode creates a new node with given identifier
func NewNode(self string) *Node {
	node := new(Node)
	node.self = self
	node.forwards = make(map[string]*Forward)
	return node
}

// SetForward on a node (insert/update)
func (n *Node) SetForward(target, next string, hops int16) {
	forward, ok := n.forwards[target]
	if !ok {
		forward = new(Forward)
		n.forwards[target] = forward
	}
	forward.next = next
	forward.hops = hops
}

// list of all nodes in the simulation
var (
	nodes = make(map[string]*Node)
)

// run application
func main() {
	log.Println("LEArn/TEAch routing analyzer")
	log.Println("(c) 2022, Bernd Fix     >Y<")

	// parse arguments
	var (
		eventLog string
		stats    string
	)
	flag.StringVar(&eventLog, "i", "", "event log (binary)")
	flag.StringVar(&stats, "s", "", "statistics output file (csv)")
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
		// read mandatory fields
		ev := new(LogEntry)
		if err = binary.Read(f, binary.BigEndian, &ev.Type); err != nil {
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
		node, ok := nodes[self]
		if !ok {
			node = NewNode(self)
			nodes[self] = node
		}
		// read additional fields depending on type
		switch ev.Type {
		case sim.EvNodeAdded:
			var idx uint16
			_ = binary.Read(f, binary.BigEndian, &ev.X)
			_ = binary.Read(f, binary.BigEndian, &ev.Y)
			_ = binary.Read(f, binary.BigEndian, &ev.R2)
			_ = binary.Read(f, binary.BigEndian, &idx)
			_ = binary.Read(f, binary.BigEndian, &ev.Running)
			_ = binary.Read(f, binary.BigEndian, &ev.Pending)
			node.idx = int(idx)
			node.x = ev.X
			node.y = ev.Y
			node.r2 = ev.R2

		case sim.EvNodeRemoved:
			_ = binary.Read(f, binary.BigEndian, &ev.Running)
			_ = binary.Read(f, binary.BigEndian, &ev.Pending)

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

	// create statistics on demand
	var csv *os.File
	var start, epoch int64
	if len(stats) > 0 {
		// create file
		if csv, err = os.Create(stats); err != nil {
			log.Fatal(err)
		}
		defer csv.Close()
		// write header
		_, _ = csv.WriteString("Epoch,Loops,Broken,Success,NumPeers,Started,StopPending,MeanHops\n")
		start = entries[0].TS
	}
	// reconstruct forward tables of node step by step
	running, started, pending := 0, 0, 0
	for _, ev := range entries {
		if csv != nil {
			// check for new epoch
			et := (ev.TS - start) / (1000000 * 5)
			if et > epoch {
				epoch = et
				res := analyzeRoutes()
				if csv != nil {
					mean := 0.
					if res.success > 0 {
						mean = float64(res.totalHops) / float64(res.success)
					}
					line := fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%.2f\n",
						epoch, res.loops, res.broken, res.success, running, started, pending, mean)
					_, _ = csv.WriteString(line)
				}
			}
		}
		// handle entry
		self := base32.StdEncoding.EncodeToString(ev.Peer[:5])[:8]
		node := nodes[self]
		ref := base32.StdEncoding.EncodeToString(ev.Ref[:5])[:8]
		switch ev.Type {
		case sim.EvNodeAdded:
			running = int(ev.Running)
			pending = int(ev.Pending)
			started++

		case sim.EvNodeRemoved:
			running = int(ev.Running)
			pending = int(ev.Pending)

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
			delete(nodes, ref)
		default:
			log.Fatalf("unhandled log entry type %d", ev.Type)
		}
	}
	if perf != len(nodes) {
		log.Fatal("missing performance data")
	}
	info()
}

func info() {
	// traffic statistics and mean number of neighbors
	mIn, mOut := 0., 0.
	dIn, dOut := 0., 0.
	neighbors := 0
	for _, node := range nodes {
		mIn += float64(node.traffIn)
		mOut += float64(node.traffOut)
		for _, f := range node.forwards {
			if f.next == "" {
				neighbors++
			}
		}
	}
	num := float64(len(nodes))
	meanNb := float64(neighbors) / num
	mIn /= num
	mOut /= num
	for _, node := range nodes {
		dIn += math.Abs(float64(node.traffIn) - mIn)
		dOut += math.Abs(float64(node.traffOut) - mOut)
	}
	dIn /= num
	dOut /= num
	log.Printf("Traffic per peer (%d peers): %s ±%s in, %s ±%s out",
		len(nodes),
		sim.Scale(mIn), sim.Scale(dIn),
		sim.Scale(mOut), sim.Scale(dOut))
	dIn /= meanNb
	dOut /= meanNb
	mIn /= meanNb
	mOut /= meanNb
	log.Printf("Traffic per neighbor: (%.2f neighbors): %s ±%s in, %s ±%s out",
		meanNb,
		sim.Scale(mIn), sim.Scale(dIn),
		sim.Scale(mOut), sim.Scale(dOut))

	// run analysis
	log.Printf("Analyzing routes between %d peers:", len(nodes))
	res := analyzeRoutes()
	analyzeLoops(res)
	analyzeBroken(res)

	total := num * (num - 1)
	if total > 0 {
		perc := func(n int) float64 {
			return float64(100*n) / total
		}
		log.Printf("  * Loops: %d (%.2f%%)", res.loops, perc(res.loops))
		log.Printf("  * Broken: %d (%.2f%%)", res.broken, perc(res.broken))
		log.Printf("  * Success: %d (%.2f%%)", res.success, perc(res.success))
		if res.success > 0 {
			mean := float64(res.totalHops) / float64(res.success)
			log.Printf("  * Hops (routg): %.2f (%d)", mean, res.success)
		}
	}
}
