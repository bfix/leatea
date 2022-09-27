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
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

//----------------------------------------------------------------------

// Node represents a node in the network
type Node struct {
	// ForwardTable as base type
	ForwardTable

	prv   *PeerPrivate // private signing key
	inCh  chan Message // channel for incoming messages
	outCh chan Message // channel for outgoing messages

	// Node running?
	// I know: "Share memory by communicating; don't communicate by
	// sharing memory.", but: just a signal whether the receiver is
	// still alive seems an excusable exception.
	active atomic.Bool
}

// NewNode creates a new node with a given private signing key and an input /
// output channel pair to send and receive messages.
func NewNode(prv *PeerPrivate, in, out chan Message, debug bool) *Node {
	pub := prv.Public()
	return &Node{
		ForwardTable: *NewForwardTable(pub, debug),
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

// Start the node (with periodic tasks and message handling)
func (n *Node) Start(ctx context.Context, notify Listener) {
	// remember listener for events
	n.listener = notify

	// start forward table
	n.ForwardTable.Start()

	// broadcast LEARN message periodically
	learn := time.NewTicker(time.Duration(cfg.LearnIntv) * time.Second)
	beacon := time.NewTicker(time.Duration(cfg.BeaconIntv) * time.Second)
	n.active.Store(true)
	for n.active.Load() {
		select {
		case <-ctx.Done():
			// termination requested
			n.active.Store(false)
			return

		case <-beacon.C:
			// send out beacon message
			msg := NewBeaconMsg(n.self)
			n.send(msg)

		case <-learn.C:
			// send out our own learn message
			msg := n.NewLearn()
			n.send(msg)
			// notify listener
			if notify != nil {
				notify(&Event{
					Type: EvWantToLearn,
					Peer: n.self,
					Val:  msg,
				})
			}

		case msg := <-n.inCh:
			// handle incoming message
			go n.Receive(msg)
		}
	}
}

// Stop a running node
func (n *Node) Stop() {
	// flag as removed
	n.active.Store(false)
	n.ForwardTable.Stop()
}

// IsRunning returns true if the node is active
func (n *Node) IsRunning() bool {
	return n.active.Load()
}

// Receive handles an incoming message
func (n *Node) Receive(msg Message) {
	// stop receiving messages on a non-running node
	if !n.active.Load() {
		return
	}
	// add the sender as direct neighbor to the
	// forward table.
	sender := msg.Sender()
	n.AddNeighbor(sender)

	// handle received message
	switch msg.Type() {
	//------------------------------------------------------------------
	// Beacon received
	//------------------------------------------------------------------
	case MsgBeacon:
		// no actions

	//------------------------------------------------------------------
	// LEArn message received
	//------------------------------------------------------------------
	case MsgLEArn:
		// assemble teach message
		m, _ := msg.(*LEArnMsg)
		out, counts := n.Teach(m)
		if out != nil {
			n.send(out)

			// notify listener
			if n.listener != nil {
				n.listener(&Event{
					Type: EvTeaching,
					Peer: n.self,
					Ref:  m.Sender(),
					Val:  []any{out, counts},
				})
			}
		}

	//------------------------------------------------------------------
	// TEAch message received
	//------------------------------------------------------------------
	case MsgTEAch:
		// learn new peers
		m, _ := msg.(*TEAchMsg)
		n.Learn(m)

		// notify listener
		if n.listener != nil {
			n.listener(&Event{
				Type: EvLearning,
				Peer: n.self,
				Ref:  m.Sender(),
				Val:  m,
			})
		}
	}
}

// String returns a human-readable representation of the node
func (n *Node) String() string {
	return fmt.Sprintf("Node{%s: [%d]}", n.self, n.NumForwards())
}
