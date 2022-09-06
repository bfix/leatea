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
	"testing"
)

func TestIntersect(t *testing.T) {
	wall := &Line{
		From: &Position{X: 30, Y: 50},
		To:   &Position{X: 70, Y: 50},
	}
	num := 0
	blocked := 0
	for i := 0.; i <= 100.; i += 5. {
		num++
		line := &Line{
			From: &Position{X: 50, Y: 0},
			To:   &Position{X: 50 - 2*(i-50), Y: 100},
		}
		rc := line.Intersect(wall)
		if rc {
			blocked++
		}
		t.Logf("%2d -- %v\n", int(i), rc)
	}
	t.Logf("Blocked %d from %d\n", blocked, num)
}
