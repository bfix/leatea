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

import "github.com/bfix/gospel/data"

const (
	MSG_LEARN = 1 // LEARN message type
	MSG_TEACH = 2 // TEACH message type
)

// Message interface
type Message interface {
	Type() int
	Sender() *Node
}

type MessageImpl struct {
	sender *Node
}

func (m *MessageImpl) Sender() *Node {
	return m.sender
}

//----------------------------------------------------------------------

type LearnMsg struct {
	MessageImpl
	pf *data.BloomFilter
}

func (m *LearnMsg) Type() int {
	return MSG_LEARN
}

//----------------------------------------------------------------------

type TeachMsg struct {
	MessageImpl
	announce []*Entry
}

func (m *TeachMsg) Add(e *Entry) {
	m.announce = append(m.announce, e)
}

func (m *TeachMsg) Type() int {
	return MSG_TEACH
}
