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
	"leatea/sim"
	"math"

	svg "github.com/ajstarks/svgo"
)

//----------------------------------------------------------------------
// Model with circular node layout (evenly spaced) with reach just
// spanning the two neighbors
//----------------------------------------------------------------------

// CircModel for special circular layout
type CircModel struct{}

// Connectivity between two nodes only based on reach (interface impl)
func (m *CircModel) Connectivity(n1, n2 *sim.SimNode) bool {
	return n1.CanReach(n2) || n2.CanReach(n1)
}

// Placement decides where to place i.th node with calculated reach (interface impl)
func (m *CircModel) Placement(i int) (r2 float64, pos *sim.Position) {
	rad := math.Max(sim.Cfg.Env.Length, sim.Cfg.Env.Width) / 2
	alpha := 2 * math.Pi / float64(sim.Cfg.Env.NumNodes)
	reach := 1.2 * rad * math.Tan(alpha)
	pos = &sim.Position{
		X: sim.Cfg.Env.Width/2 + rad*math.Cos(float64(i)*alpha),
		Y: sim.Cfg.Env.Length/2 + rad*math.Sin(float64(i)*alpha),
	}
	r2 = reach * reach
	return
}

// Draw the environment
func (m *CircModel) Draw(svg *svg.SVG, xlate func(x float64) int) {}

//----------------------------------------------------------------------

// Get the "physical" environment that controls connectivity
func getEnvironment(env *sim.EnvironCfg) sim.Environment {
	switch env.Class {
	case "rand":
		return new(sim.RndModel)
	case "circ":
		return new(CircModel)
	case "wall":
		mdl := sim.NewWallModel()
		for _, wall := range env.Walls {
			mdl.Add(
				&sim.Position{X: wall.X1, Y: wall.Y1},
				&sim.Position{X: wall.X2, Y: wall.Y2},
				wall.F)
		}
		return mdl
	}
	return nil
}
