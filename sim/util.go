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
	"fmt"
	"time"
)

//----------------------------------------------------------------------

// Position (2D)
type Position struct {
	x, y float64
}

// Distance2 returns the squared distance between positions.
func (p *Position) Distance2(pos *Position) float64 {
	dx := p.x - pos.x
	dy := p.y - pos.y
	return dx*dx + dy*dy
}

// String returns a human-readable representation
func (p *Position) String() string {
	return fmt.Sprintf("(%.2f,%.2f)", p.x, p.y)
}

//----------------------------------------------------------------------

var _scales = []string{"B", "kB", "MB", "GB"}

// Scale returns a byte size as a string in compact form
func Scale(v float64) string {
	var pos int
	for pos = 0; pos < len(_scales); pos++ {
		if v < 1024. {
			return fmt.Sprintf("%.2f%s", v, _scales[pos])
		}
		v /= 1024.
	}
	return fmt.Sprintf("%.2f%s", v, _scales[pos-1])
}

// Vary a time span 't'
func Vary(t float64) time.Duration {
	v := Random.ExpFloat64() * t
	return time.Duration(v*1000) * time.Millisecond
}
