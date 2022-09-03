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

import "leatea/core"

type Route struct {
	hops []int
}

func (r *Route) Clone() *Route {
	return &Route{
		hops: core.Clone(r.hops),
	}
}

func (r *Route) Contains(n int) bool {
	for _, h := range r.hops {
		if h == n {
			return true
		}
	}
	return false
}

func (r *Route) Add(n int) {
	r.hops = append(r.hops, n)
}

func (r *Route) Hops() int {
	return len(r.hops) - 1
}

type Graph struct {
	mdl map[int]*Route
}

func NewGraph() *Graph {
	return &Graph{
		mdl: make(map[int]*Route),
	}
}

func (g *Graph) NewRoute() *Route {
	return &Route{
		hops: make([]int, 0),
	}
}

func (g *Graph) ShortestPath(start int, end int) *Route {
	return g.shortestPath(start, end, g.NewRoute())
}

func (g *Graph) shortestPath(start int, end int, p *Route) (r *Route) {
	if _, ok := g.mdl[start]; !ok {
		return nil
	}
	r = p.Clone()
	r.Add(start)
	if start == end {
		return r
	}
	var shortest *Route
	for _, node := range g.mdl[start].hops {
		if !p.Contains(node) {
			newPath := g.shortestPath(node, end, r)
			if newPath == nil {
				continue
			}
			if shortest == nil || (len(newPath.hops) < len(shortest.hops)) {
				shortest = newPath
			}
		}
	}
	return shortest
}
