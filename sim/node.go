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
	"leatea/core"
	"log"
	"math"
)

//----------------------------------------------------------------------

// SimNode represents a node in the test network (extended attributes)
type SimNode struct {
	core.Node
	pos  *Position         // position in the field
	v    float64           // velocity (in units per epoch)
	dir  float64           // direction [0,2π(
	r2   float64           // square of broadcast distance
	recv chan core.Message // channel for incoming messages
}

// NewSimNode creates a new node in the test network
func NewSimNode(prv *core.PeerPrivate, out chan core.Message, pos *Position, r2 float64) *SimNode {
	recv := make(chan core.Message)
	return &SimNode{
		Node: *core.NewNode(prv, recv, out),
		r2:   r2,
		pos:  pos,
		recv: recv,
	}
}

// Stop the node
func (n *SimNode) Stop(running int) {
	n.Node.Stop()
	log.Printf("Node %s stopped (%d running)", n.Node.PeerID(), running)
}

// CanReach returns true if the node can reach another node by broadcast
func (n *SimNode) CanReach(peer *SimNode) bool {
	dist2 := n.pos.Distance2(peer.pos)
	return dist2 < n.r2
}

// Receive a message and process it
func (n *SimNode) Receive(msg core.Message) {
	n.recv <- msg
}

// String returns a human-readable representation.
func (n *SimNode) String() string {
	return fmt.Sprintf("SimNode{%s @ %s}", n.Node.String(), n.pos)
}

// Draw a node on the canvas
func (n *SimNode) Draw(c Canvas) {
	r := math.Sqrt(n.r2)
	c.Circle(n.pos.X, n.pos.Y, 0.3, 0, nil, ClrRed)
	c.Circle(n.pos.X, n.pos.Y, r, 0.03, ClrBlack, nil)
	c.Text(n.pos.X, n.pos.Y+1.3, 1, n.PeerID().String(), "middle")
}
