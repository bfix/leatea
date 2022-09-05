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
	"time"
)

//----------------------------------------------------------------------
// Network simulation to test the LEATEA algorithm
//----------------------------------------------------------------------

// Environent defines the connectivity between two nodes based on the
// "phsical" model of the environment.
type Environment func(n1, n2 *SimNode) bool

// Placement describes how to place i.th node.
type Placement func(i int) (r2 float64, pos *Position)

// Network is the overall test controller
type Network struct {
	env     Environment         // model of the environment
	nodes   map[string]*SimNode // list of nodes (keyed by peerid)
	queue   chan core.Message   // "ether" for message transport
	trafOut uint64              // total "send" traffic
	trafIn  uint64              // total "receive" traffic
	active  bool                // simulation running?
}

// NewNetwork creates a new network of 'numNodes' randomly distributed nodes
// in an area of 'width x length'. All nodes have the same squared broadcast
// range r2.
func NewNetwork(put Placement, env Environment) *Network {
	n := new(Network)
	n.env = env
	n.queue = make(chan core.Message, 10)
	n.nodes = make(map[string]*SimNode)
	// create and run nodes.
	for i := 0; i < NumNodes; i++ {
		r2, pos := put(i)
		prv := core.NewPeerPrivate()
		delay := Vary(BootupTime)
		node := NewSimNode(prv, n.queue, pos, r2, delay)
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
				if n.env(node, sender) && !node.PeerID().Equal(sender.PeerID()) {
					// node in reach receives message
					n.trafIn += mSize
					go node.Receive(msg)
				}
			}
		}
	}
}

// Stop the network (message exchange)
func (n *Network) Stop() int {
	// stop all nodes
	for _, node := range n.nodes {
		if node.IsRunning() {
			node.Stop()
		}
	}
	// stop network
	n.active = false

	// discard messages in queue
	discard := 0
	wdog := time.NewTicker(5 * time.Second)
loop:
	for {
		select {
		case <-n.queue:
			discard++
		case <-wdog.C:
			break loop
		}
	}
	return discard
}

//----------------------------------------------------------------------
// Analysis helpers
//----------------------------------------------------------------------

// Coverage returns the mean coverage of all forward tables (known targets)
func (n *Network) Coverage() float64 {
	total := 0
	num := activeNodes
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
func (n *Network) RoutingTable() ([][]int, *Graph, float64) {
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
	// construct graph
	g := NewGraph()
	for k1, node1 := range n.nodes {
		i1 := index[k1]
		neighbors := make([]int, 0)
		for k2, node2 := range n.nodes {
			if k1 == k2 || !node1.CanReach(node2) {
				continue
			}
			i2 := index[k2]
			neighbors = append(neighbors, i2)
		}
		g.mdl[i1] = neighbors
	}
	// return results
	return res, g, float64(allHops) / float64(numRoute)
}
