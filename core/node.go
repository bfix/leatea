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
	ForwardTable // forward table as base type

	prv      *PeerPrivate // private signing key
	inCh     chan Message // channel for incoming messages
	outCh    chan Message // channel for outgoing messages
	active   bool         // node running?
	listener Listener     // listener for node events
}

// NewNode creates a new node with a given private signing key and an input /
// output channel pair to send and receive messages.
func NewNode(prv *PeerPrivate, in, out chan Message) *Node {
	pub := prv.Public()
	return &Node{
		ForwardTable: *NewForwardTable(pub),
		prv:          prv,
		inCh:         in,
		outCh:        out,
	}
}

// PeerID returns the peerid of the node.
func (n *Node) PeerID() *PeerID {
	return n.self
}

// Send message (to outgoing message channel)
func (n *Node) send(msg Message) {
	go func() {
		n.outCh <- msg
	}()
}

// Run the node (with periodic tasks and message handling)
func (n *Node) Run(notify Listener) {
	// reset routing table (in case the node is restarted)
	n.Reset()

	// remember listener for events
	n.listener = notify

	// broadcast LEARN message periodically
	learn := time.NewTicker(time.Duration(cfg.LearnIntv) * time.Second)
	n.active = true
	for n.active {
		select {
		case <-learn.C:
			// node already stopped?
			if !n.active {
				return
			}
			// send out our own learn message
			if notify != nil {
				notify(&Event{
					Type: EvBeacon,
					Peer: n.self,
				})
			}
			msg := NewLearnMsg(n.self, n.Filter(notify))
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
	n.AddNeighbor(sender)

	// handle received message
	switch msg.Type() {

	//------------------------------------------------------------------
	// LEArn message received
	//------------------------------------------------------------------
	case MsgLEArn:
		m, _ := msg.(*LEArnMsg)
		// build a list of candidate entries for teaching:
		// candidates are not included in the learn filter
		// and don't have the learner as next hop.
		if candidates := n.Candidates(m); len(candidates) > 0 {
			// assemble and send TEACH message
			msg := NewTEAchMsg(n.self, candidates)
			n.send(msg)
			if n.listener != nil {
				n.listener(&Event{
					Type: EvTeaching,
					Peer: m.Sender(),
				})
			}
		}

	//------------------------------------------------------------------
	// TEAch message received
	//------------------------------------------------------------------
	case MsgTEAch:
		m, _ := msg.(*TEAchMsg)
		if n.listener != nil {
			n.listener(&Event{
				Type: EvLearning,
				Peer: n.self,
			})
		}
		// learn new peers
		n.Learn(m)
	}
}

// String returns a human-readable representation of the node
func (n *Node) String() string {
	return fmt.Sprintf("Node{%s: [%d]}", n.self, n.NumForwards())
}
