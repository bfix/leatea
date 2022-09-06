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
	rad := math.Max(sim.Length, sim.Width) / 2
	alpha := 2 * math.Pi / float64(sim.NumNodes)
	reach := 1.2 * rad * math.Tan(alpha)
	pos = &sim.Position{
		X: sim.Width/2 + rad*math.Cos(float64(i)*alpha),
		Y: sim.Length/2 + rad*math.Sin(float64(i)*alpha),
	}
	r2 = reach * reach
	return
}

// Draw the environment
func (m *CircModel) Draw(svg *svg.SVG, xlate func(x float64) int) {}

//----------------------------------------------------------------------

// Get the "physical" environment that controls connectivity
func getEnvironment(env string) sim.Environment {
	switch env {
	case "rand":
		return new(sim.RndModel)
	case "circ":
		return new(CircModel)
	case "cross":
		mdlCross := sim.NewWallModel()
		mdlCross.Add(
			&sim.Position{X: sim.Width / 3, Y: sim.Length / 2},
			&sim.Position{X: 2 * sim.Width / 3, Y: sim.Length / 2},
			0.)
		mdlCross.Add(
			&sim.Position{X: sim.Width / 2, Y: sim.Length / 3},
			&sim.Position{X: sim.Width / 2, Y: 2 * sim.Length / 3},
			0.)
		return mdlCross
	case "divide":
		mdlCross := sim.NewWallModel()
		mdlCross.Add(
			&sim.Position{X: sim.Width / 3, Y: sim.Length / 2},
			&sim.Position{X: sim.Width, Y: sim.Length / 2},
			0.)
		return mdlCross
	}
	return nil
}
