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
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"time"
)

//----------------------------------------------------------------------
// Time
//----------------------------------------------------------------------

// Time is the number of microseconds since Jan 1st, 1970 (Unix epoch)
type Time struct {
	Val int64 `order:"big"`
}

// TimeNow returns the current time
func TimeNow() *Time {
	return &Time{Val: time.Now().UnixMicro()}
}

//----------------------------------------------------------------------
// Random numbers
//----------------------------------------------------------------------

// RndUInt64 returns a random uint64 integer
func RndUInt64() uint64 {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	var v uint64
	c := bytes.NewBuffer(b)
	_ = binary.Read(c, binary.BigEndian, &v)
	return v
}

// RndUInt64 returns a random uint32 integer
func RndUInt32() uint32 {
	return uint32(RndUInt64())
}

//----------------------------------------------------------------------
// generic array helpers
//----------------------------------------------------------------------

// Clone creates a new array of same content as the argument.
func Clone[T []E, E any](d T) T {
	// handle nil slices
	if d == nil {
		return nil
	}
	// create copy
	r := make(T, len(d))
	copy(r, d)
	return r
}

// Equal returns true if two arrays match.
func Equal[T []E, E comparable](a, b T) bool {
	if len(a) != len(b) {
		return false
	}
	for i, e := range a {
		if e != b[i] {
			return false
		}
	}
	return true
}

// Reverse the content of an array
func Reverse[T []E, E any](b T) T {
	bl := len(b)
	r := make(T, bl)
	for i := 0; i < bl; i++ {
		r[bl-i-1] = b[i]
	}
	return r
}

// IsAll returns true if all elements in an array are set to null.
func IsAll[T []E, E comparable](b T, null E) bool {
	for _, v := range b {
		if v != null {
			return false
		}
	}
	return true
}

// Fill an array with a value
func Fill[T []E, E any](b T, val E) {
	for i := range b {
		b[i] = val
	}
}
