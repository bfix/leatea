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

// Event types
const (
	EvBeacon          = 1 // sending out LEARN message
	EvLearning        = 2 // received TEACH message, learning peers
	EvTeaching        = 3 // sending out TEACH message
	EvNeighborExpired = 4 // neighbor expired
	EvForwardRemoved  = 5 // forward removed from routing table
)

// Event from network if something interesting happens
type Event struct {
	Type int     // event tpe (see consts)
	Peer *PeerID // peer identifier
	Ref  *PeerID // reference peer (optinal)
	Val  int     // additional data
}

// Listener for network events
type Listener func(*Event)
