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
	EvWantToLearn = 1 // sending out LEARN message
	EvLearning    = 2 // received TEACH message, learning peers
	EvTeaching    = 3 // sending out TEACH message

	EvForwardLearned = 10 // new forward learned
	EvForwardChanged = 11 // change in the forward table

	EvNeighborExpired = 20 // neighbor expired
	EvNeighborAdded   = 21 // new neighbor added
	EvNeighborUpdated = 22 // old neighbor updated
	EvNeighborRelayed = 23 // dormant neighbor revived as relay

	EvRelayRemoved = 30 // relay removed from routing table
	EvRelayRevived = 31 // dormant relay revived (with new forward)
	EvRelayUpdated = 32 // relay updated
	EvShorterRoute = 33 // shorter path for forward entry found

	EvLoopDetect = 40 // loop construction detected
)

// Event from network if something interesting happens
type Event struct {
	Type int     // event type (see consts)
	Seq  uint32  // sequence number (per node)
	Peer *PeerID // peer identifier
	Ref  *PeerID // reference peer (optinal)
	Val  any     // additional data
}

// GetVal returns a type value from an event
func GetVal[T any](ev *Event) (val T) {
	val, _ = ev.Val.(T)
	return val
}

// Listener for network events
type Listener func(*Event)
