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

import "time"

// Config for LEArn/TEAch core processes
type Config struct {
	MaxTeachs int           `json:"maxTeach"`  // max. number of entries in TEACH message
	TTLEntry  time.Duration `json:"ttlEntry"`  // time to live for a table entry (neighbor)
	LearnIntv time.Duration `json:"learnIntv"` // LEARN intervall
}

// package-local configuration data (with default values)
var cfg = &Config{
	MaxTeachs: 10,
	TTLEntry:  time.Minute,
	LearnIntv: 10 * time.Second,
}

// SetConfiguration before use
func SetConfiguration(c *Config) {
	if c.MaxTeachs > 0 {
		cfg.MaxTeachs = c.MaxTeachs
	}
	if c.TTLEntry > 0 {
		cfg.TTLEntry = c.TTLEntry
	}
	if c.LearnIntv > 0 {
		cfg.LearnIntv = c.LearnIntv
	}
}
