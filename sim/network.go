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

type Network struct {
	nodes   map[string]*SimNode
	queue   chan core.Message
	reach2  float64
	trafOut uint64
	trafIn  uint64
	active  bool
}

func NewNetwork(numNodes int, width, length, r2 float64) *Network {
	n := new(Network)
	n.queue = make(chan core.Message, 10)
	n.nodes = make(map[string]*SimNode)
	n.reach2 = r2
	for i := 0; i < numNodes; i++ {
		prv := core.NewPeerPrivate()
		node := NewSimNode(prv, n.queue, &Position{
			x: rand.Float64() * width,
			y: rand.Float64() * length,
		})
		n.nodes[node.PeerID().Key()] = node
		go node.Run()
	}
	return n
}

func (n *Network) Run() {
	n.active = true
	for n.active {
		msg := <-n.queue
		mSize := uint64(msg.Size())
		n.trafOut += mSize
		if sender, ok := n.nodes[msg.Sender().Key()]; ok {
			for _, node := range n.nodes {
				dist2 := node.pos.Distance2(sender.pos)
				if dist2 < n.reach2 && !node.PeerID().Equal(sender.PeerID()) {
					n.trafIn += mSize
					go node.Receive(msg)
				}
			}
		}
	}
}

func (n *Network) Stop() {
	n.active = false
}

func (n *Network) Coverage() float64 {
	total := 0
	num := len(n.nodes)
	for _, node := range n.nodes {
		total += node.Forwards()
	}
	return float64(100*total) / float64(num*(num-1))
}

func (n *Network) Traffic() (in, out uint64) {
	return n.trafIn, n.trafOut
}

func (n *Network) FullTable() ([][]int, float64) {
	allHops := 0
	numRoute := 0
	num := len(n.nodes)
	res := make([][]int, num)
	for i := range res {
		res[i] = make([]int, num)
	}
	look := make(map[string]int)
	pos := 0
	for k := range n.nodes {
		look[k] = pos
		pos++
	}
	for k1, node1 := range n.nodes {
		i1 := look[k1]
		for k2, node2 := range n.nodes {
			if k1 == k2 {
				res[i1][i1] = -2
				continue
			}
			i2 := look[k2]
			if next, hops := node1.Forward(node2.PeerID()); hops > 0 {
				allHops += hops
				numRoute++
				ref := i2
				if next != nil {
					ref = look[next.Key()]
				}
				res[i1][i2] = ref
			} else {
				res[i1][i2] = -1
			}
		}
	}
	return res, float64(allHops) / float64(numRoute)
}
