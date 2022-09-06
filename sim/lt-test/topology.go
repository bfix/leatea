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
	"math/rand"
)

// Node placement configurations
var nodePlacer = map[string]sim.Placement{
	"rand": func(i int) (r2 float64, pos *sim.Position) {
		pos = &sim.Position{
			X: rand.Float64() * sim.Width,  //nolint:gosec // deterministic testing
			Y: rand.Float64() * sim.Length, //nolint:gosec // deterministic testing
		}
		r2 = sim.Reach2
		return
	},
	"circ": func(i int) (r2 float64, pos *sim.Position) {
		rad := math.Max(sim.Length, sim.Width) / 2
		alpha := 2 * math.Pi / float64(sim.NumNodes)
		reach := rad * math.Tan(alpha)
		pos = &sim.Position{
			X: sim.Width/2 + rad*math.Cos(float64(i)*alpha),
			Y: sim.Length/2 + rad*math.Sin(float64(i)*alpha),
		}
		r2 = reach * reach
		return
	},
}

//----------------------------------------------------------------------

func getEnvironment(env string) sim.Connectivity {
	switch env {
	case "open":
		return func(n1, n2 *sim.SimNode) bool {
			return n1.CanReach(n2) || n2.CanReach(n1)
		}
	case "cross":
		mdlCross := sim.NewWallModel()
		mdlCross.Add(
			&sim.Position{X: 30, Y: 50},
			&sim.Position{X: 70, Y: 50},
			0.)
		/*
			mdlCross.Add(
				&sim.Position{X: sim.Width / 3, Y: sim.Length / 2},
				&sim.Position{X: 2 * sim.Width / 3, Y: sim.Length / 2},
				0.)
			mdlCross.Add(
				&sim.Position{X: sim.Width / 2, Y: sim.Length / 3},
				&sim.Position{X: sim.Width / 2, Y: 2 * sim.Length / 3},
				0.)
		*/
		return mdlCross.CanReach
	}
	return nil
}
