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
func (g *Graph) SVG(wrt io.Writer, final bool) {
	// find longest reach for offset
	reach := 0.
	for _, node := range g.netw.nodes {
		if node.r2 > reach {
			reach = node.r2
		}
	}
	// start generating SVG
	canvas := NewSVGCanvas(wrt, Cfg.Env.Width, Cfg.Env.Height, math.Sqrt(reach))

	// draw environment
	g.netw.env.Draw(canvas)

	// draw nodes
	list := make([]*SimNode, len(g.netw.nodes))
	for key, node := range g.netw.nodes {
		if !final && !node.IsRunning() {
			continue
		}
		id := g.netw.index[key]
		list[id] = node
		node.Draw(canvas)
	}
	// draw connections
	for key, node := range g.netw.nodes {
		if node == nil || (!final && !node.IsRunning()) {
			continue
		}
		id := g.netw.index[key]
		for _, n := range g.mdl[id] {
			node2 := list[n]
			if node2 == nil || (!final && !node2.IsRunning()) {
				continue
			}
			canvas.Line(node.pos.X, node.pos.Y, node2.pos.X, node2.pos.Y, 0.15, ClrBlack)
		}
	}
	canvas.End()
}
