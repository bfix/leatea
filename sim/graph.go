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
	"io"
	"math"

	svg "github.com/ajstarks/svgo"
)

// Graph is a list of nodes that have a list of neighbors. The graph is
// independently constructed from nodes and their positions and is not
// based on results of the routing algorithm it is going to check.
type Graph struct {
	mdl  map[int][]int
	netw *Network
}

// NewGraph creates a new graph instance
func NewGraph(n *Network) *Graph {
	return &Graph{
		mdl:  make(map[int][]int),
		netw: n,
	}
}

// Compute a distance vector from start node to all other nodes it can reach
// (Dijkstra shortest path algorithm)
func (g *Graph) Distance(start int) (dist []int) {
	num := len(g.mdl)
	spt := make([]bool, num)
	dist = make([]int, num)
	for i := range dist {
		dist[i] = math.MaxInt
	}
	dist[start] = 0
	for {
		min := math.MaxInt
		best := -1
		for i, d := range dist {
			if d < min && !spt[i] {
				min = d
				best = i
			}
		}
		if best == -1 {
			return
		}
		spt[best] = true
		d := dist[best] + 1
		for _, v := range g.mdl[best] {
			if d < dist[v] {
				dist[v] = d
			}
		}
	}
}

// SVG creates an image of the graph
func (g *Graph) SVG(wrt io.Writer) {
	// find longest reach for offset
	reach := 0.
	for _, node := range g.netw.nodes {
		if node.r2 > reach {
			reach = node.r2
		}
	}
	off := math.Sqrt(reach)
	xlate := func(xy float64) int {
		return int((xy + off) * 100)
	}
	// start generating SVG
	canvas := svg.New(wrt)
	canvas.Start(xlate(Cfg.Env.Width+2*off), xlate(Cfg.Env.Length+2*off))

	// draw environment
	g.netw.env.Draw(canvas, xlate)

	// draw nodes
	list := make([]*SimNode, len(g.netw.nodes))
	for key, node := range g.netw.nodes {
		if !node.IsRunning() {
			continue
		}
		x1 := xlate(node.pos.X)
		y1 := xlate(node.pos.Y)
		r := int(math.Sqrt(node.r2) * 100)
		id := g.netw.index[key]
		list[id] = node
		canvas.Circle(x1, y1, 50, "fill:red")
		canvas.Circle(x1, y1, r, "stroke:black;stroke-width:3;fill:none")
		canvas.Text(x1, y1+130, node.PeerID().String(), "text-anchor:middle;font-size:100px")
	}
	// draw connections
	for key, node1 := range g.netw.nodes {
		if node1 == nil || !node1.IsRunning() {
			continue
		}
		x1 := xlate(node1.pos.X)
		y1 := xlate(node1.pos.Y)
		id := g.netw.index[key]
		for _, n := range g.mdl[id] {
			node2 := list[n]
			if node2 == nil || !node2.IsRunning() {
				continue
			}
			x2 := xlate(node2.pos.X)
			y2 := xlate(node2.pos.Y)
			canvas.Line(x1, y1, x2, y2, "stroke:black;stroke-width:15")
		}
	}
	canvas.End()
}
