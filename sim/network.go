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
	"leatea/core"
	"math/rand"
)

//----------------------------------------------------------------------
// Network simulation to test the LEATEA algorithm
//----------------------------------------------------------------------

// Network is the overall test controller
type Network struct {
	nodes   map[string]*SimNode // list of nodes (keyed by peerid)
	queue   chan core.Message   // "ether" for message transport
	trafOut uint64              // total "send" traffic
	trafIn  uint64              // total "receive" traffic
	active  bool                // simulation running?
}

// NewNetwork creates a new network of 'numNodes' randomly distributed nodes
// in an area of 'width x length'. All nodes have the same squared broadcast
// range r2.
func NewNetwork(numNodes int, width, length, r2 float64) *Network {
	n := new(Network)
	n.queue = make(chan core.Message, 10)
	n.nodes = make(map[string]*SimNode)
	// create and run nodes.
	for i := 0; i < numNodes; i++ {
		prv := core.NewPeerPrivate()
		pos := &Position{rand.Float64() * width, rand.Float64() * length}
		node := NewSimNode(prv, r2, n.queue, pos)
		n.nodes[node.PeerID().Key()] = node
		go node.Run()
	}
	return n
}

// Run the network simulation
func (n *Network) Run() {
	n.active = true
	for n.active {
		// wait for broadcasted message.
		msg := <-n.queue
		mSize := uint64(msg.Size())
		n.trafOut += mSize
		// lookup sender in node table
		if sender, ok := n.nodes[msg.Sender().Key()]; ok {
			// process all nodes that are in broadcast reach of the sender
			for _, node := range n.nodes {
				if sender.CanReach(node) && !node.PeerID().Equal(sender.PeerID()) {
					// node in reach receives message
					n.trafIn += mSize
					go node.Receive(msg)
				}
			}
		}
	}
}

// Stop the network (message exchange)
func (n *Network) Stop() {
	n.active = false
}

//----------------------------------------------------------------------
// Analysis helpers
//----------------------------------------------------------------------

// Coverage returns the mean coverage of all forward tables (known targets)
func (n *Network) Coverage() float64 {
	total := 0
	num := len(n.nodes)
	for _, node := range n.nodes {
		total += node.NumForwards()
	}
	return float64(100*total) / float64(num*(num-1))
}

// Traffic returns traffic volumes (in and out)
func (n *Network) Traffic() (in, out uint64) {
	return n.trafIn, n.trafOut
}

// RoutingTable returns the routing table for the whole
// network and the average number of hops.
func (n *Network) RoutingTable() ([][]int, float64) {
	allHops := 0
	numRoute := 0
	// create empty routing table
	num := len(n.nodes)
	res := make([][]int, num)
	for i := range res {
		res[i] = make([]int, num)
	}
	// index maps a peerid to an integer
	index := make(map[string]int)
	pos := 0
	for k := range n.nodes {
		index[k] = pos
		pos++
	}
	for k1, node1 := range n.nodes {
		i1 := index[k1]
		for k2, node2 := range n.nodes {
			if k1 == k2 {
				res[i1][i1] = -2 // "self" route
				continue
			}
			i2 := index[k2]
			if next, hops := node1.Forward(node2.PeerID()); hops > 0 {
				allHops += hops
				numRoute++
				ref := i2
				if next != nil {
					ref = index[next.Key()]
				}
				res[i1][i2] = ref
			} else {
				res[i1][i2] = -1
			}
		}
	}
	// return results
	return res, float64(allHops) / float64(numRoute)
}
