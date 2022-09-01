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
	"fmt"
	"leatea/core"
)

type SimNode struct {
	core.Node
	pos  *Position
	recv chan core.Message
}

func NewSimNode(prv *core.PeerPrivate, out chan core.Message, pos *Position) *SimNode {
	recv := make(chan core.Message)
	return &SimNode{
		Node: *core.NewNode(prv, recv, out),
		pos:  pos,
		recv: recv,
	}
}

func (n *SimNode) Receive(msg core.Message) {
	n.recv <- msg
}

func (n *SimNode) String() string {
	return fmt.Sprintf("SimNode{%s @ %s}", n.Node.String(), n.pos)
}
