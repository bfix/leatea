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
	"sync"
	"time"
)

// Event types for network events
const (
	EvNodeAdded   = 100 // node added to network
	EvNodeRemoved = 101 // node removed from network
)

//----------------------------------------------------------------------
// Network simulation to test the LEATEA algorithm
//----------------------------------------------------------------------

// Network is the overall test controller
type Network struct {
	env Environment // model of the environment

	index map[string]int   // node index map
	nodes map[int]*SimNode // list of nodes
	lock  sync.RWMutex     // manage access to nodes

	running int               // number of running nodes
	queue   chan core.Message // "ether" for message transport
	trafOut uint64            // total "send" traffic
	trafIn  uint64            // total "receive" traffic
	active  bool              // simulation running?
	cb      core.Listener     // listener for network events
}

// NewNetwork creates a new network of 'numNodes' randomly distributed nodes
// in an area of 'width x height'. All nodes have the same squared broadcast
// range r2.
func NewNetwork(env Environment) *Network {
	n := new(Network)
	n.env = env
	n.queue = make(chan core.Message, 10)
	n.nodes = make(map[int]*SimNode)
	n.index = make(map[string]int)
	n.running = 0
	return n
}

// GetShortID returns a short identifier for a node.
func (n *Network) GetShortID(p *core.PeerID) int {
	if p == nil {
		return 0
	}
	i, ok := n.index[p.Key()]
	if !ok {
		return -1
	}
	return i
}

// Run the network simulation
func (n *Network) Run(cb core.Listener) {
	n.active = true

	// create and run nodes.
	n.cb = cb
	for i := 0; i < Cfg.Env.NumNodes; i++ {
		r2, pos := n.env.Placement(i)
		prv := core.NewPeerPrivate()
		delay := Vary(Cfg.Node.BootupTime)
		node := NewSimNode(prv, n.queue, pos, r2)

		// run node (delayed)
		go func(i int) {
			time.Sleep(delay)
			if n.active {
				// register node with environment and get an integer identifier.
				idx := n.env.Register(i, node)
				// add node to network
				n.lock.Lock()
				n.index[node.PeerID().Key()] = idx
				n.nodes[idx] = node
				n.running++
				n.lock.Unlock()

				// notify listener
				if cb != nil {
					cb(&core.Event{
						Type: EvNodeAdded,
						Peer: node.PeerID(),
						Val:  []int{idx, n.running},
					})
				}
				// run node
				node.Run(cb)
			}
		}(i)
		// shutdown node (delayed)
		go func() {
			// only some peers stop working
			if Random.Float64() < Cfg.Node.DeathRate {
				ttl := Vary(Cfg.Node.PeerTTL) + delay + 2*time.Minute
				time.Sleep(ttl)
				if n.active {
					// stop node
					n.StopNode(node)
				}
			}
		}()
	}
	// simulate transport layer
	for n.active {
		// wait for broadcasted message.
		msg := <-n.queue
		mSize := uint64(msg.Size())
		n.trafOut += mSize
		// lookup sender in node table
		if sender, _ := n.getNode(msg.Sender()); sender != nil {
			// process all nodes that are in broadcast reach of the sender
			n.lock.RLock()
			for _, node := range n.nodes {
				if node.IsRunning() && n.env.Connectivity(node, sender) && !node.PeerID().Equal(sender.PeerID()) {
					// active node in reach receives message
					n.trafIn += mSize
					go node.Receive(msg)
				}
			}
			n.lock.RUnlock()
		}
	}
}

func (n *Network) IsActive() bool {
	if n == nil {
		return false
	}
	return n.active
}

func (n *Network) NumRunning() int {
	return n.running
}

func (n *Network) StopNodeByID(p *core.PeerID) int {
	node, _ := n.getNode(p)
	if node == nil {
		panic("stop node by id: no node")
	}
	return n.StopNode(node)
}

// StopNode request
func (n *Network) StopNode(node *SimNode) int {
	if node.IsRunning() {
		// stop running node
		n.running--
		node.Stop()

		// notify listener
		if n.cb != nil {
			n.cb(&core.Event{
				Type: EvNodeRemoved,
				Peer: node.PeerID(),
				Val:  []int{node.id, n.running},
			})
		}
	}
	return n.running
}

// Stop the network (message exchange)
func (n *Network) Stop() int {
	// stop all nodes
	remain := len(n.nodes)
	for _, node := range n.nodes {
		remain--
		if node.IsRunning() {
			n.StopNode(node)
		}
	}
	// stop network
	n.active = false

	// discard pending messages in queue
	discard := 0
	wdog := time.NewTicker(time.Duration(Cfg.Env.CoolDown) * time.Second)
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

func (n *Network) getNode(p *core.PeerID) (node *SimNode, idx int) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	var ok bool
	if idx, ok = n.index[p.Key()]; !ok {
		return
	}
	node = n.nodes[idx]
	return
}

//----------------------------------------------------------------------
// Analysis helpers
//----------------------------------------------------------------------

// Traffic returns traffic volumes (in and out)
func (n *Network) Traffic() (in, out uint64) {
	return n.trafIn, n.trafOut
}

// RoutingTable returns the routing table for the whole
// network and the average number of hops.
func (n *Network) RoutingTable() (*RoutingTable, float64) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	// create new routing table and graph
	rt := NewRoutingTable()

	// add nodes to routing table
	for i, node := range n.nodes {
		rt.AddNode(i, node)
	}

	// build routing table and graph
	allHops := 0
	numRoute := 0

	for i1, node1 := range n.nodes {
		if !node1.IsRunning() {
			continue
		}
		for i2, node2 := range n.nodes {
			if !node2.IsRunning() {
				continue
			}
			if i1 == i2 {
				continue
			}
			if next, hops := node1.Forward(node2.PeerID()); hops > 0 {
				allHops += hops
				numRoute++
				ref := i2
				if next != nil {
					ref = rt.Index[next.Key()]
				}
				rt.List[i1].Forwards[i2] = ref
			}
		}
	}
	// return results
	return rt, float64(allHops) / float64(numRoute)
}

// Render the network directly.
func (n *Network) Render(c Canvas) {
	n.lock.RLock()
	defer n.lock.RUnlock()

	// render nodes and connections
	for i1, node1 := range n.nodes {
		if node1.IsRunning() {
			node1.Draw(c)
		}
		for _, id2 := range node1.Neighbors() {
			node2, i2 := n.getNode(id2)
			if node2 == nil {
				continue
			}
			// check that an inactive node1 has a forward from active node2
			if !node1.IsRunning() && node2.IsRunning() {
				if _, hops := node2.Forward(node1.PeerID()); hops == 0 {
					continue
				}
			}
			// don't draw if both nodes are inactive
			r1 := node1.IsRunning()
			r2 := node2.IsRunning()
			//			if i2 >= i1 || (n.active && !(node2.IsRunning() || node1.IsRunning())) {
			if i2 >= i1 || (n.active && !(r1 || r2)) {
				continue
			}
			clr := ClrBlue
			if n.active && !(node2.IsRunning() && node1.IsRunning()) {
				clr = ClrRed
			}
			c.Line(node1.Pos.X, node1.Pos.Y, node2.Pos.X, node2.Pos.Y, 0.15, clr)
		}
	}
	// draw environment
	n.env.Draw(c)
}
