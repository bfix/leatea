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
	"encoding/json"
	"fmt"
	"leatea/core"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Environment interface {
	// Connectivity between two nodes based on the "phsical" model
	// of the environment.
	Connectivity(n1, n2 *SimNode) bool

	// Placement decides where to place i.th node with calculated reach.
	Placement(i int) (r2 float64, pos *Position)

	// Register node with environment
	Register(i int, node *SimNode) int

	// Epoch started
	Epoch(epoch int) []*core.Event

	// Draw the environment
	Draw(Canvas)
}

//----------------------------------------------------------------------
// Model with "walls" that block connectivity
//----------------------------------------------------------------------

// WallModel for walls with opacity
type WallModel struct {
	walls []*Wall // list of all walls in the world
}

// NewWallModel returns an empty model for walls
func NewWallModel() *WallModel {
	return &WallModel{
		walls: make([]*Wall, 0),
	}
}

// Connectivity between two nodes based on a wall model (interface impl)
func (m *WallModel) Connectivity(n1, n2 *SimNode) bool {
	los := &Line{n1.Pos, n2.Pos}
	red := 1.0
	for _, w := range m.walls {
		if w.Line.Intersect(los) {
			red *= w.reduce
		}
	}
	if red < 1e-8 {
		return false
	}
	d2 := n1.Pos.Distance2(n2.Pos) / red
	return n1.r2 > d2 || n2.r2 > d2
}

// Placement decides where to place i.th node with calculated reach (interface impl)
func (m *WallModel) Placement(i int) (r2 float64, pos *Position) {
	pos = &Position{
		X: Random.Float64() * Cfg.Env.Width,
		Y: Random.Float64() * Cfg.Env.Height,
	}
	r2 = Cfg.Node.Reach2
	return
}

// Register node with environment
func (m *WallModel) Register(i int, node *SimNode) int {
	node.id = i + 1
	return node.id
}

// Epoch started
func (m *WallModel) Epoch(epoch int) []*core.Event {
	return nil
}

// Draw the environment
func (m *WallModel) Draw(c Canvas) {
	for _, wall := range m.walls {
		c.Line(wall.From.X, wall.From.Y, wall.To.X, wall.To.Y, 0.7, ClrBlack)
	}
}

// Add a new wall
func (m *WallModel) Add(from, to *Position, red float64) {
	wall := new(Wall)
	wall.From = from
	wall.To = to
	wall.reduce = red
	m.walls = append(m.walls, wall)
}

// Wall with opacity: reach is reduced by factor
type Wall struct {
	Line
	reduce float64
}

// Line in 2D space
type Line struct {
	From *Position
	To   *Position
}

// Intersect returns true if to segments intersect.
func (l *Line) Intersect(t *Line) bool {
	return l.Side(t.From)*l.Side(t.To) == -1 && t.Side(l.From)*t.Side(l.To) == -1
}

// Side returns -1 for left, 1 for right side and 0 for "on line"
func (l *Line) Side(p *Position) int {
	z := (p.X-l.From.X)*(l.To.Y-l.From.Y) - (p.Y-l.From.Y)*(l.To.X-l.From.X)
	if math.Abs(z) < 1e-8 {
		return 0
	}
	if z < 0 {
		return -1
	}
	return 1
}

//----------------------------------------------------------------------
// Simple model with random distribution
//----------------------------------------------------------------------

// WallModel for walls with opacity
type RndModel struct{}

// Connectivity between two nodes only based on reach (interface impl)
func (m *RndModel) Connectivity(n1, n2 *SimNode) bool {
	d2 := n1.Pos.Distance2(n2.Pos)
	return n1.r2 > d2 || n2.r2 > d2
}

// Placement decides where to place i.th node with calculated reach (interface impl)
func (m *RndModel) Placement(i int) (r2 float64, pos *Position) {
	pos = &Position{
		X: Random.Float64() * Cfg.Env.Width,
		Y: Random.Float64() * Cfg.Env.Height,
	}
	r2 = Cfg.Node.Reach2
	return
}

// Register node with environment
func (m *RndModel) Register(i int, node *SimNode) int {
	node.id = i + 1
	return node.id
}

// Epoch started
func (m *RndModel) Epoch(epoch int) []*core.Event {
	return nil
}

// Draw the environment
func (m *RndModel) Draw(Canvas) {}

//----------------------------------------------------------------------
// Model with circular node layout (evenly spaced) with reach just
// spanning the two neighbors
//----------------------------------------------------------------------

// CircModel for special circular layout
type CircModel struct{}

// Connectivity between two nodes only based on reach (interface impl)
func (m *CircModel) Connectivity(n1, n2 *SimNode) bool {
	return n1.CanReach(n2) || n2.CanReach(n1)
}

// Placement decides where to place i.th node with calculated reach (interface impl)
func (m *CircModel) Placement(i int) (r2 float64, pos *Position) {
	rad := math.Max(Cfg.Env.Height, Cfg.Env.Width) / 2
	alpha := 2 * math.Pi / float64(Cfg.Env.NumNodes)
	reach := 1.2 * rad * math.Tan(alpha)
	pos = &Position{
		X: Cfg.Env.Width/2 + rad*math.Cos(float64(i)*alpha),
		Y: Cfg.Env.Height/2 + rad*math.Sin(float64(i)*alpha),
	}
	r2 = reach * reach
	return
}

// Register node with environment
func (m *CircModel) Register(i int, node *SimNode) int {
	node.id = i + 1
	return node.id
}

// Epoch started
func (m *CircModel) Epoch(epoch int) []*core.Event {
	return nil
}

// Draw the environment
func (m *CircModel) Draw(Canvas) {}

//----------------------------------------------------------------------
// Model with explicit links
//----------------------------------------------------------------------

// LinkedNodes for a LinkModel
type LinkedNode struct {
	n *SimNode
	d *NodeDef
}

// LinkModel ia list of nodes with explicit connections (neighbors)
type LinkModel struct {
	nodes map[int]*LinkedNode
	defs  []*NodeDef
	ids   map[string]int
}

func NewLinkModel() *LinkModel {
	return &LinkModel{
		nodes: make(map[int]*LinkedNode),
		ids:   make(map[string]int),
	}
}

// Connectivity between two nodes based on link (interface impl)
func (m *LinkModel) Connectivity(n1, n2 *SimNode) bool {
	check := func(n *SimNode, t int) bool {
		for _, l := range m.nodes[n.id].d.Links {
			if l == t {
				return true
			}
		}
		return false
	}
	rc1 := check(n1, n2.id)
	rc2 := check(n2, n1.id)
	if rc1 != rc2 {
		log.Printf("%d -> %d: %v", n1.id, n2.id, rc1)
		log.Printf("%d: %v", n1.id, m.nodes[n1.id].d.Links)
		log.Printf("%d -> %d: %v", n2.id, n1.id, rc2)
		log.Printf("%d: %v", n2.id, m.nodes[n2.id].d.Links)
		panic("")
	}
	return rc1
}

// Placement decides where to place i.th node (interface impl)
func (m *LinkModel) Placement(i int) (r2 float64, pos *Position) {
	def := m.defs[i]
	return 0, &Position{def.X, def.Y}
}

// Register node with environment
func (m *LinkModel) Register(i int, node *SimNode) int {
	node.id = m.defs[i].ID
	m.nodes[node.id].n = node
	m.ids[node.PeerID().Key()] = node.id
	return node.id
}

// Epoch started
func (m *LinkModel) Epoch(epoch int) (events []*core.Event) {
	// check if nodes expire
	for _, def := range m.defs {
		if def.TTL == epoch {
			// stop node
			node := m.nodes[def.ID].n
			events = append(events, &core.Event{
				Type: EvNodeRemoved,
				Peer: node.PeerID(),
				Val:  []int{node.id, -1},
			})
		}
	}
	// show forward tables
	show := func(p *core.PeerID) string {
		if p == nil {
			return "0"
		}
		return strconv.Itoa(m.ids[p.Key()])
	}
	list := make([]string, 0)
	for _, ln := range m.nodes {
		if ln.n == nil || !ln.n.IsRunning() {
			continue
		}
		tbl := ln.n.TableList(show)
		list = append(list, fmt.Sprintf("[%d] Tbl = %s", ln.n.id, tbl))
	}
	// sort list by ascending peer
	sort.Slice(list, func(i, j int) bool {
		s1 := list[i][1:strings.Index(list[i], "]")]
		s2 := list[j][1:strings.Index(list[j], "]")]
		if len(s1) < len(s2) {
			return true
		}
		if len(s1) > len(s2) {
			return false
		}
		return s1 < s2
	})
	// print table
	for _, out := range list {
		log.Println(out)
	}
	/*
		// show all routes
		for i1, n1 := range m.nodes {
			if !n1.n.IsRunning() {
				continue
			}
			for i2, n2 := range m.nodes {
				if !n2.n.IsRunning() {
					continue
				}
				if i1 == i2 {
					continue
				}
				var route []int
				from := n1.n
				to := n2.n.PeerID()
				ttl := len(m.nodes)
				hops := 0
				for {
					route = append(route, from.id)
					next, steps := from.Forward(to)
					if steps == 0 {
						route = append(route, -1)
						break
					}
					if next == nil {
						if steps == 1 {
							route = append(route, i2)
						} else {
							route = append(route, -2)
						}
						break
					}
					from = m.nodes[m.ids[next.Key()]].n
					if hops++; hops > ttl {
						route = append(route, -3)
						break
					}
				}
				log.Printf("[%d --> %d]: %v", i1, i2, route)
			}
		}
	*/
	return
}

// Draw the environment
func (m *LinkModel) Draw(Canvas) {}

//----------------------------------------------------------------------

// BuildEnvironment: create the "physical" environment that
// controls connectivity and movement of nodes
func BuildEnvironment(env *EnvironCfg) Environment {
	switch env.Class {

	//------------------------------------------------------------------
	// Random distribution of env.NumNodes over given area
	//------------------------------------------------------------------
	case "rand":
		return new(RndModel)

	//------------------------------------------------------------------
	// Evenly space env.NumNodes nodes on a circle so that each node
	// only reaches its two direct neighbors (Shortcut for calculating
	// nodes in a LinkModel)
	//------------------------------------------------------------------
	case "circ":
		return new(CircModel)

	//------------------------------------------------------------------
	// Randomly distributed nodes over given area with obstacles (walls)
	//------------------------------------------------------------------
	case "wall":
		mdl := NewWallModel()
		for _, wall := range env.Walls {
			mdl.Add(
				&Position{X: wall.X1, Y: wall.Y1},
				&Position{X: wall.X2, Y: wall.Y2},
				wall.F)
		}
		return mdl
	//------------------------------------------------------------------
	// Use explicit node definitions and connectivity
	//------------------------------------------------------------------
	case "link":
		mdl := NewLinkModel()
		// get node definitions
		if len(env.NodesRef) != 0 {
			// we read nodes from a file
			buf, err := os.ReadFile(env.NodesRef)
			if err != nil {
				log.Fatal(err)
			}
			// read node definitions
			if err = json.Unmarshal(buf, &mdl.defs); err != nil {
				log.Fatal(err)
			}
		} else {
			mdl.defs = env.Nodes
		}
		Cfg.Env.NumNodes = len(mdl.defs)
		// create linked nodes
		for _, def := range mdl.defs {
			mdl.nodes[def.ID] = &LinkedNode{d: def}
		}
		return mdl
	}
	return nil
}
