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
	"io"
	"leatea/core"
	"log"
	"math"
	"time"

	svg "github.com/ajstarks/svgo"
)

//----------------------------------------------------------------------
// Network simulation to test the LEATEA algorithm
//----------------------------------------------------------------------

// Network is the overall test controller
type Network struct {
	env     Environment         // model of the environment
	nodes   map[string]*SimNode // list of nodes (keyed by peerid)
	running int                 // number of running nodes
	index   map[string]int      // node index map
	queue   chan core.Message   // "ether" for message transport
	trafOut uint64              // total "send" traffic
	trafIn  uint64              // total "receive" traffic
	active  bool                // simulation running?
}

// NewNetwork creates a new network of 'numNodes' randomly distributed nodes
// in an area of 'width x length'. All nodes have the same squared broadcast
// range r2.
func NewNetwork(env Environment) *Network {
	n := new(Network)
	n.env = env
	n.queue = make(chan core.Message, 10)
	n.nodes = make(map[string]*SimNode)
	n.index = make(map[string]int)
	n.running = 0
	// create and run nodes.
	for i := 0; i < NumNodes; i++ {
		r2, pos := env.Placement(i)
		prv := core.NewPeerPrivate()
		delay := Vary(BootupTime)
		node := NewSimNode(prv, n.queue, pos, r2)
		key := node.PeerID().Key()
		n.nodes[key] = node
		n.index[key] = i
		go func() {
			time.Sleep(delay)
			n.running++
			log.Printf("Node %s started (#%d)", node.Node.PeerID(), n.running)
			node.Node.Run()
		}()
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
				if n.env.Connectivity(node, sender) && !node.PeerID().Equal(sender.PeerID()) {
					// node in reach receives message
					n.trafIn += mSize
					go node.Receive(msg)
				}
			}
		}
	}
}

// Botted returns true if all nodes have started
func (n *Network) Booted() bool {
	return n.running == len(n.nodes)
}

// Stop the network (message exchange)
func (n *Network) Stop() int {
	// stop all nodes
	remain := len(n.nodes)
	for _, node := range n.nodes {
		remain--
		if node.IsRunning() {
			n.running--
			node.Stop(remain)
		}
	}
	// stop network
	n.active = false

	// discard messages in queue
	discard := 0
	wdog := time.NewTicker(CoolDown)
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
func (n *Network) RoutingTable() (*RoutingTable, *Graph, float64) {
	allHops := 0
	numRoute := 0
	rt := NewRoutingTable(n)
	// index maps a peerid to an integer
	for k1, node1 := range n.nodes {
		i1 := n.index[k1]
		for k2, node2 := range n.nodes {
			if k1 == k2 {
				rt.List[i1][i1] = -2 // "self" route
				continue
			}
			i2 := n.index[k2]
			if next, hops := node1.Forward(node2.PeerID()); hops > 0 {
				allHops += hops
				numRoute++
				ref := i2
				if next != nil {
					ref = n.index[next.Key()]
				}
				rt.List[i1][i2] = ref
			} else {
				rt.List[i1][i2] = -1
			}
		}
	}
	// construct graph
	g := NewGraph(n)
	for k1, node1 := range n.nodes {
		i1 := n.index[k1]
		neighbors := make([]int, 0)
		for k2, node2 := range n.nodes {
			if k1 == k2 || !n.env.Connectivity(node1, node2) {
				continue
			}
			i2 := n.index[k2]
			neighbors = append(neighbors, i2)
		}
		g.mdl[i1] = neighbors
	}
	// return results
	return rt, g, float64(allHops) / float64(numRoute)
}

type RoutingTable struct {
	netw *Network
	List [][]int
}

func NewRoutingTable(n *Network) *RoutingTable {
	// create empty routing table
	num := len(n.nodes)
	res := make([][]int, num)
	for i := range res {
		res[i] = make([]int, num)
	}
	return &RoutingTable{
		netw: n,
		List: res,
	}
}

// SVG creates an image of the graph
func (rt *RoutingTable) SVG(wrt io.Writer) {
	// find longest reach for offset
	reach := 0.
	for _, node := range rt.netw.nodes {
		if node.r2 > reach {
			reach = node.r2
		}
	}
	off := math.Sqrt(reach)
	xlate := func(xy float64) int {
		return int((xy + off) * 100)
	}
	// start generating SVG
	canvas := svg.New(wrt)
	canvas.Start(xlate(Width+2*off), xlate(Length+2*off))

	// draw environment
	rt.netw.env.Draw(canvas, xlate)

	// draw nodes
	list := make([]*SimNode, len(rt.netw.nodes))
	for key, node := range rt.netw.nodes {
		cx := xlate(node.pos.X)
		cy := xlate(node.pos.Y)
		r := int(math.Sqrt(node.r2) * 100)
		id := rt.netw.index[key]
		list[id] = node
		canvas.Circle(cx, cy, 50, "fill:red")
		canvas.Circle(cx, cy, r, "stroke:black;stroke-width:3;fill:none")
		canvas.Text(cx, cy+120, node.PeerID().String(), "text-anchor:middle;font-size:100px")
	}
	// draw connections
	for from, neighbors := range rt.List {
		node1 := list[from]
		x1 := xlate(node1.pos.X)
		y1 := xlate(node1.pos.Y)
		for _, to := range neighbors {
			if to < 0 {
				continue
			}
			node2 := list[to]
			x2 := xlate(node2.pos.X)
			y2 := xlate(node2.pos.Y)
			canvas.Line(x1, y1, x2, y2, "stroke:blue;stroke-width:15")
		}
	}
	canvas.End()
}
