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
	"fmt"
	"time"
)

//----------------------------------------------------------------------

// Node represents a node in the network
type Node struct {
	prv    *PeerPrivate  // private signing key
	pub    *PeerID       // peer identifier (public key)
	rt     *ForwardTable // forward table
	inCh   chan Message  // channel for incoming messages
	outCh  chan Message  // channel for outgoing messages
	active bool          // node running?
}

// NewNode creates a new node with a given private signing key and an input /
// output channel pair to send and receive messages.
func NewNode(prv *PeerPrivate, in, out chan Message) *Node {
	pub := prv.Public()
	return &Node{
		prv:   prv,
		pub:   pub,
		rt:    NewForwardTable(pub),
		inCh:  in,
		outCh: out,
	}
}

// PeerID returns the peerid of the node.
func (n *Node) PeerID() *PeerID {
	return n.pub
}

// Send message (to outgoing message channel)
func (n *Node) send(msg Message) {
	go func() {
		n.outCh <- msg
	}()
}

// NumForwards returns the number of targets in the forward table
func (n *Node) NumForwards() int {
	return len(n.rt.list)
}

// Forward returns the next hop on the route to target and the number of
// expected hops. If hop count is less than 0, a next hop doesn't exist
// (broken route)
func (n *Node) Forward(target *PeerID) (*PeerID, int) {
	return n.rt.Forward(target)
}

// Run the node (with periodic tasks and message handling)
func (n *Node) Run() {
	// broadcast LEARN message periodically
	learn := time.NewTicker(learnIntv)
	n.active = true
	for n.active {
		select {
		case <-learn.C:
			// send out our own learn message
			msg := NewLearnMsg(n.pub, n.rt.Filter())
			n.send(msg)

		case msg := <-n.inCh:
			// handle incoming message
			go n.Receive(msg)
		}
	}
}

// Stop a running node
func (n *Node) Stop() {
	n.active = false
}

// IsRunning returns true if the node is active
func (n *Node) IsRunning() bool {
	return n.active
}

// Receive handles an incoming message
func (n *Node) Receive(msg Message) {
	// add the sender as direct neighbor to the
	// forward table.
	sender := msg.Sender()
	e := &Entry{
		Forward: Forward{
			Peer: sender,
			Hops: 0,
		},
		NextHop:  nil,
		LastSeen: TimeNow(),
	}
	n.rt.Add(e)

	// handle received message
	switch msg.Type() {

	//------------------------------------------------------------------
	// LEARN message received
	//------------------------------------------------------------------
	case MsgLEARN:
		m, _ := msg.(*LearnMsg)
		// build a list of candidate entries for teaching:
		// candidates are not included in the learn filter
		// and don't have the learner as next hop.
		if candidates := n.rt.Candidates(m); len(candidates) > 0 {
			// assemble and send TEACH message
			msg := NewTeachMsg(n.pub, candidates)
			n.send(msg)
		}

	//------------------------------------------------------------------
	// TEACH message received
	//------------------------------------------------------------------
	case MsgTEACH:
		m, _ := msg.(*TeachMsg)
		// learn new peers
		n.rt.Learn(m)
	}
}

// String returns a human-readable representation of the node
func (n *Node) String() string {
	return fmt.Sprintf("Node{%s: [%d]}", n.pub, len(n.rt.list))
}
