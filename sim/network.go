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
	"context"
	"leatea/core"
	"math/rand"
	"sync"
	"sync/atomic"
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

	// Node management
	index    map[string]int   // node index map
	nodes    map[int]*SimNode // list of nodes
	nodeLock sync.RWMutex     // manage access to nodes

	// Transport layer
	queue chan core.Message // "ether" for message transport

	// State of the network
	active   atomic.Bool  // simulation running?
	check    atomic.Bool  // sanity check running?
	statLock sync.RWMutex // manage access to status fields
	running  int          // number of running nodes
	started  int          // number of started nodes
	removals int          // number of pending removals

	// Listener for network events
	cb core.Listener
}

// NewNetwork creates a new network of 'numNodes' in a given environment.
func NewNetwork(env Environment, numNodes int) *Network {
	n := new(Network)
	n.env = env
	n.queue = make(chan core.Message)
	n.nodes = make(map[int]*SimNode)
	n.index = make(map[string]int)
	n.running = 0
	n.started = 0
	n.removals = 0
	n.active.Store(false)
	return n
}

// GetShortID returns a short identifier for a node.
func (n *Network) GetShortID(p *core.PeerID) int {
	n.nodeLock.RLock()
	defer n.nodeLock.RUnlock()

	// handle nil receiver
	if p == nil {
		return 0
	}
	// lookup id in index
	id, ok := n.index[p.Key()]
	if !ok {
		return -1
	}
	return id
}

// Run the network simulation
func (n *Network) Run(ctx context.Context, cb core.Listener) {
	n.active.Store(true)

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
			if n.active.Load() {
				// register node with environment and get an integer identifier.
				idx := n.env.Register(i, node)
				// add node to network
				n.nodeLock.Lock()
				n.index[node.PeerID().Key()] = idx
				n.nodes[idx] = node
				n.nodeLock.Unlock()

				// update status
				n.statLock.Lock()
				n.started++
				n.running++
				running := n.running
				n.statLock.Unlock()

				// notify listener
				if cb != nil {
					cb(&core.Event{
						Type: EvNodeAdded,
						Peer: node.PeerID(),
						Val:  []int{idx, running},
					})
				}
				// run node
				node.Start(ctx, cb)
			}
		}(i)
		// shutdown node (delayed)
		go func() {
			// only some peers stop working
			if rand.Float64() < Cfg.Node.DeathRate { //nolint:gosec // deterministic testing
				n.statLock.Lock()
				n.removals++
				n.statLock.Unlock()
				ttl := Vary(Cfg.Node.PeerTTL) + delay + 2*time.Minute
				time.Sleep(ttl)
				if n.active.Load() {
					// stop node
					n.StopNode(node)
				}
				n.statLock.Lock()
				n.removals--
				n.statLock.Unlock()
			}
		}()
	}
	// simulate transport layer
	n.check.Store(false)
	for n.active.Load() {
		select {
		// requested termination
		case <-ctx.Done():
			return

		// wait for broadcasted message.
		case msg := <-n.queue:
			// lookup sender in node table
			if sender, _ := n.getNode(msg.Sender()); sender != nil {
				// add message to sender output
				sender.traffOut.Add(uint64(msg.Size()))

				// process all nodes that are in broadcast reach of the sender
				n.nodeLock.RLock()
				for _, node := range n.nodes {
					if node.IsRunning() && n.env.Connectivity(node, sender) && !node.PeerID().Equal(sender.PeerID()) {
						// active node in reach receives message
						go node.Receive(msg)
					}
				}
				n.nodeLock.RUnlock()
			}
			// call sanity check (not stacking)
			go n.sanityCheck()
		}
	}
}

func (n *Network) IsActive() bool {
	if n == nil {
		return false
	}
	return n.active.Load()
}

func (n *Network) Settled() bool {
	if n == nil {
		return false
	}
	n.statLock.RLock()
	defer n.statLock.RUnlock()
	return n.started == Cfg.Env.NumNodes && n.removals == 0
}

func (n *Network) Stats() (int, int, int) {
	n.statLock.RLock()
	defer n.statLock.RUnlock()
	return n.running, n.started, n.removals
}

func (n *Network) Nodes() (list []*SimNode) {
	n.nodeLock.RLock()
	defer n.nodeLock.RUnlock()

	for _, node := range n.nodes {
		list = append(list, node)
	}
	return
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
		n.statLock.Lock()
		n.running--
		running := n.running
		node.Stop()
		n.statLock.Unlock()

		// notify listener
		if n.cb != nil {
			n.cb(&core.Event{
				Type: EvNodeRemoved,
				Peer: node.PeerID(),
				Val:  []int{node.id, running},
			})
		}
		return running
	}
	return -1
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
	n.active.Store(false)

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
	n.nodeLock.RLock()
	defer n.nodeLock.RUnlock()

	if p == nil {
		return
	}
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

// RoutingTable returns the routing table for the whole
// network and the average number of hops.
func (n *Network) RoutingTable() (rt *RoutingTable) {
	n.nodeLock.RLock()
	defer n.nodeLock.RUnlock()

	// create new routing table
	rt = NewRoutingTable()

	// add nodes to routing table
	for i, node := range n.nodes {
		if node.IsRunning() {
			rt.AddNode(i, node)
		}
	}

	// build routing table
	for i1, e1 := range rt.List {
		for i2, e2 := range n.nodes {
			if i1 == i2 {
				continue
			}
			if next, hops := e1.Node.Forward(e2.Node.PeerID()); hops > 0 {
				ref := i2
				if next != nil {
					ref = rt.Index[next.Key()]
				}
				rt.List[i1].Forwards[i2] = ref
			}
		}
	}
	return
}

// Render the network directly.
func (n *Network) Render(c Canvas) {
	n.nodeLock.RLock()
	defer n.nodeLock.RUnlock()

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
			if i2 >= i1 || (n.active.Load() && !(r1 || r2)) {
				continue
			}
			clr := ClrBlue
			if n.active.Load() && !(node2.IsRunning() && node1.IsRunning()) {
				clr = ClrRed
			}
			c.Line(node1.Pos.X, node1.Pos.Y, node2.Pos.X, node2.Pos.Y, 0.15, clr)
		}
	}
	// draw environment
	n.env.Draw(c)
}

func (n *Network) sanityCheck() {
	if n.check.Load() {
		return
	}
	// do the sanity check...
	n.check.Store(false)
}
