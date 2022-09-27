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

package sim

import (
	"leatea/core"
	"log"
	"os"

	"github.com/bfix/gospel/data"
)

//----------------------------------------------------------------------
// Routing table
//----------------------------------------------------------------------

// RTEntry is an entry in the routing table defining a node and its forwards
type RTEntry struct {
	Node     *SimNode
	Forwards map[int]int
}

// RoutingTable for network (spanning all nodes)
type RoutingTable struct {
	List  map[int]*RTEntry
	Index map[string]int
}

func NewRoutingTable() *RoutingTable {
	// create empty routing table
	return &RoutingTable{
		List:  make(map[int]*RTEntry),
		Index: make(map[string]int),
	}
}

func (rt *RoutingTable) AddNode(i int, node *SimNode) {
	rt.Index[node.PeerID().Key()] = i
	rt.List[i] = &RTEntry{
		Node:     node,
		Forwards: make(map[int]int),
	}
}

func (rt *RoutingTable) Status() (loops, broken, success, totalHops int) {
	for from := range rt.List {
		for to := range rt.List {
			if from == to {
				continue
			}
			hops, _ := rt.Route(from, to)
			switch hops {
			case -1:
				loops++
			case 0:
				broken++
			default:
				totalHops += hops
				success++
			}
		}
	}
	return
}

// Follow the route to target. Returns number of hops on success, 0 for
// broken routes and -1 for cycles.
func (rt *RoutingTable) Route(from, to int) (hops int, route []int) {
	ttl := len(rt.Index)
	hops = 0
	for {
		route = append(route, from)
		entry, ok := rt.List[from]
		if !ok {
			hops = 0
			return
		}
		hops++
		next := entry.Forwards[to]
		if next == to {
			route = append(route, to)
			return
		}
		if next < 0 {
			//log.Printf("%d --> %d: %v", from, to, route)
			hops = 0
			return
		}
		from = next
		if ttl--; ttl < 0 {
			hops = -1
			return
		}
	}
}

// Render creates an image of the graph
func (rt *RoutingTable) Render(canvas Canvas) {
	for _, entry := range rt.List {
		// draw node
		nodeFrom := entry.Node
		nodeFrom.Draw(canvas)

		// draw connections
		for _, next := range entry.Forwards {
			nodeTo := rt.List[next].Node
			canvas.Line(nodeFrom.Pos.X, nodeFrom.Pos.Y, nodeTo.Pos.X, nodeTo.Pos.Y, 0.15, ClrBlue)
		}
	}
}

//----------------------------------------------------------------------
// Dump routing table
//----------------------------------------------------------------------

type DumpEntry struct {
	Peer uint16 `order:"big"`
	Hops int16  `order:"big"`
	Next uint16 `order:"big"`
	Age_ int64  `order:"big"`
}

func (e *DumpEntry) Age() float64 {
	return core.Age{Val: e.Age_}.Seconds()
}

type DumpNode struct {
	ID      uint16       `order:"big"`
	Running bool         ``
	Traffic uint64       `order:"big"`
	NumTbl  uint16       `order:"big"`
	Tbl     []*DumpEntry `size:"NumTbl"`
}

type Dump struct {
	NumNodes uint16      `order:"big"`
	Nodes    []*DumpNode `size:"NumNodes"`
}

func (n *Network) DumpRouting(fname string) {
	// create dump file
	f, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// build dump instance
	nodes := n.Nodes()
	dump := &Dump{
		NumNodes: uint16(len(nodes)),
		Nodes:    make([]*DumpNode, 0),
	}
	for _, node := range nodes {
		fw := make([]*DumpEntry, 0)
		for _, entry := range node.Forwards(false) {
			_, peer := n.getNode(entry.Peer)
			_, next := n.getNode(entry.NextHop)
			de := &DumpEntry{
				Peer: uint16(peer),
				Hops: entry.Hops,
				Next: uint16(next),
				Age_: entry.Origin.Age().Val,
			}
			fw = append(fw, de)
		}
		dn := &DumpNode{
			ID:      uint16(node.id),
			Running: node.IsRunning(),
			Traffic: node.traffIn.Load(),
			NumTbl:  uint16(len(fw)),
			Tbl:     fw,
		}
		dump.Nodes = append(dump.Nodes, dn)
	}
	// serialize to file
	if err = data.MarshalStream(f, dump); err != nil {
		log.Fatal(err)
	}
}
