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

package main

import (
	"math/rand"
)

type Network struct {
	nodes []*Node
	queue []Message
}

func NewNetwork() *Network {
	n := new(Network)
	n.queue = make([]Message, 0)
	n.nodes = make([]*Node, numNode)
	for i := 0; i < numNode; i++ {
		n.nodes[i] = NewNode(n, &Position{
			x: rand.Float64() * width,
			y: rand.Float64() * length,
		})
	}
	return n
}

type Callback func(int, int, int, int, int)

func (n *Network) Run(num int, cb Callback) {
	for e := 1; e < num; e++ {
		queue := n.queue
		n.queue = make([]Message, 0)
		learned := 0
		total := 0
		for _, node := range n.nodes {
			for _, msg := range queue {
				if node.pos.Distance2(msg.Sender().pos) < reach2 {
					node.Receive(msg)
				}
			}
			learned += node.Epoch()
			total += len(node.rt)
		}
		cb(e, len(queue), learned, total, len(n.queue))
		/*
			if len(queue) > 0 && learned == 0 {
				log.Printf("Complete at epoch #%d", e)
				break
			}
		*/
	}
}

func (n *Network) Broadcast(msg Message) {
	n.queue = append(n.queue, msg)
}
