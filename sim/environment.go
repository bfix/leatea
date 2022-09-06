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
	"math"
	"math/rand"
)

type Environment interface {
	// Connectivity between two nodes based on the "phsical" model
	// of the environment.
	Connectivity(n1, n2 *SimNode) bool

	// Placement decides where to place i.th node with calculated reach.
	Placement(i int) (r2 float64, pos *Position)
}

//----------------------------------------------------------------------
// Model with "walls" that block connectivity
//----------------------------------------------------------------------

// WallModel for walls with opacity
type WallModel struct {
	walls []*Wall // list of all walls in the world
}

// NewWallModel returns an empty model for walls
func NewWallModel() *WallModel {
	return &WallModel{
		walls: make([]*Wall, 0),
	}
}

// Connectivity between two nodes based on a wall model (interface impl)
func (m *WallModel) Connectivity(n1, n2 *SimNode) bool {
	los := &Line{n1.pos, n2.pos}
	red := 1.0
	for _, w := range m.walls {
		if w.Line.Intersect(los) {
			red *= w.reduce
		}
	}
	if red < 1e-8 {
		return false
	}
	d2 := n1.pos.Distance2(n2.pos) / red
	return n1.r2 > d2 || n2.r2 > d2
}

// Placement decides where to place i.th node with calculated reach (interface impl)
func (m *WallModel) Placement(i int) (r2 float64, pos *Position) {
	pos = &Position{
		X: rand.Float64() * Width,  //nolint:gosec // deterministic testing
		Y: rand.Float64() * Length, //nolint:gosec // deterministic testing
	}
	r2 = Reach2
	return
}

// Add a new wall
func (m *WallModel) Add(from, to *Position, red float64) {
	wall := new(Wall)
	wall.From = from
	wall.To = to
	wall.reduce = red
	m.walls = append(m.walls, wall)
}

// Wall with opacity: reach is reduced by factor
type Wall struct {
	Line
	reduce float64
}

// Line in 2D space
type Line struct {
	From *Position
	To   *Position
}

// Intersect returns true if to segments intersect.
func (l *Line) Intersect(t *Line) bool {
	return l.Side(t.From)*l.Side(t.To) == -1 && t.Side(l.From)*t.Side(l.To) == -1
}

// Side returns -1 for left, 1 for right side and 0 for "on line"
func (l *Line) Side(p *Position) int {
	z := (p.X-l.From.X)*(l.To.Y-l.From.Y) - (p.Y-l.From.Y)*(l.To.X-l.From.X)
	if math.Abs(z) < 1e-8 {
		return 0
	}
	if z < 0 {
		return -1
	}
	return 1
}

//----------------------------------------------------------------------
// Simple model with random distribution
//----------------------------------------------------------------------

// WallModel for walls with opacity
type RndModel struct{}

// Connectivity between two nodes only based on reach (interface impl)
func (m *RndModel) Connectivity(n1, n2 *SimNode) bool {
	d2 := n1.pos.Distance2(n2.pos)
	return n1.r2 > d2 || n2.r2 > d2
}

// Placement decides where to place i.th node with calculated reach (interface impl)
func (m *RndModel) Placement(i int) (r2 float64, pos *Position) {
	pos = &Position{
		X: rand.Float64() * Width,  //nolint:gosec // deterministic testing
		Y: rand.Float64() * Length, //nolint:gosec // deterministic testing
	}
	r2 = Reach2
	return
}
