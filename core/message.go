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

package core

import (
	"fmt"

	"github.com/bfix/gospel/data"
)

const (
	MSG_LEARN = 1 // LEARN message type
	MSG_TEACH = 2 // TEACH message type
)

// Message interface
type Message interface {
	Size() uint16
	Type() uint16
	Sender() *PeerID
	String() string
}

type MessageImpl struct {
	MsgSize uint16 `order:"big"`
	MsgType uint16 `order:"big"`
}

func (m *MessageImpl) Size() uint16 {
	return m.MsgSize
}

func (m *MessageImpl) Type() uint16 {
	return m.MsgType
}

//----------------------------------------------------------------------

type LearnMsg struct {
	MessageImpl

	Sender_ *PeerID
	Filter  *data.BloomFilter
}

func (m *LearnMsg) Type() uint16 {
	return MSG_LEARN
}

func (m *LearnMsg) Sender() *PeerID {
	return m.Sender_
}

func (m *LearnMsg) String() string {
	return fmt.Sprintf("Learn{%s}", m.Sender_.Short())
}

//----------------------------------------------------------------------

type TeachMsg struct {
	MessageImpl

	Sender_  *PeerID
	Announce []*Entry
}

func (m *TeachMsg) Add(e *Entry) {
	m.Announce = append(m.Announce, e)
}

func (m *TeachMsg) Type() uint16 {
	return MSG_TEACH
}

func (m *TeachMsg) Sender() *PeerID {
	return m.Sender_
}

func (m *TeachMsg) String() string {
	return fmt.Sprintf("Teach{%s:%d}", m.Sender_.Short(), len(m.Announce))
}

const maxTeachs = 5
