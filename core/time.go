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

package core

import (
	"time"
)

//----------------------------------------------------------------------
// Time is a (local) timestamp; the peers in the network have no
// (decentralized) way to synchronize their clocks in a reliable way.
// Timing information (that is essential for LEATEA operations) is
// sent in relative times (age; positive values are backwards!) and
// computed from timestamps when a message is sent (and converted back
// when a message is received).
//----------------------------------------------------------------------

// Time is the number of microseconds since Jan 1st, 1970 (Unix epoch)
type Time struct {
	Val int64 `order:"big"`
}

// Age of the timestamp
func (t *Time) Age() *Age {
	return &Age{time.Now().UnixMicro() - t.Val}
}

// Expired returns true if 't+ttl' is in the past
func (t *Time) Expired(ttl time.Duration) bool {
	return (time.Now().UnixMicro() - t.Val) > ttl.Microseconds()
}

// Before returns true if t is before t2
func (t *Time) Before(t2 *Time) bool {
	return t.Val < t2.Val
}

// String returns a human-readabe timestamps
func (t *Time) String() string {
	return time.UnixMicro(t.Val).Format(time.RFC1123)
}

// TimeNow returns the current time
func TimeNow() *Time {
	return &Time{Val: time.Now().UnixMicro()}
}

// TimeFromAge returns a time for a given age.
func TimeFromAge(a *Age) *Time {
	return &Time{time.Now().UnixMicro() - a.Val}
}

//----------------------------------------------------------------------

// Age is a relative time in microseconds (backwards)
type Age struct {
	Val int64 `order:"big"`
}

// String returns a human-readabe timestamps
func (a *Age) String() string {
	return time.Duration(1000 * a.Val).String()
}

// Size of an age instance (binary representation)
func (a *Age) Size() uint {
	return 8
}
