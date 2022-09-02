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

//----------------------------------------------------------------------

// Message interface
type Message interface {
	Size() uint16
	Type() uint16
	Sender() *PeerID
	String() string
}

//----------------------------------------------------------------------

// MessageImpl is a generic message used in derived message implementations.
// It implements a basic set of interface methods (all except 'String()').
type MessageImpl struct {
	MsgSize uint16  `order:"big"` // total size of message
	MsgType uint16  `order:"big"` // message type
	Sender_ *PeerID ``            // sender of message
}

// Size returns the binary size of a message
func (m *MessageImpl) Size() uint16 {
	return m.MsgSize
}

// Type returns the message type
func (m *MessageImpl) Type() uint16 {
	return m.MsgType
}

// Sender returns the peer id of the message sender
func (m *MessageImpl) Sender() *PeerID {
	return m.Sender_
}

//----------------------------------------------------------------------

// Learn message: "I want to learn, and here is what I know already..."
type LearnMsg struct {
	MessageImpl

	Filter *data.SaltedBloomFilter // bloomfilter over target peerids in forward table
}

// NewLearnMsg creates a new message for a learn broadcast
func NewLearnMsg(sender *PeerID, filter *data.SaltedBloomFilter) *LearnMsg {
	msg := new(LearnMsg)
	msg.MsgType = MSG_LEARN
	msg.MsgSize = uint16(4 + sender.Size() + filter.Size())
	msg.Sender_ = sender
	msg.Filter = filter
	return msg
}

// String returns a human-readable representation of the message
func (m *LearnMsg) String() string {
	return fmt.Sprintf("Learn{%s}", m.Sender_)
}

//----------------------------------------------------------------------

// Teach message: "This is what I know and you don't..."
type TeachMsg struct {
	MessageImpl

	Announce []*Entry `size:"*"` // unfiltered table entries
}

// NewTeachMsg creates a new message for broadcast
func NewTeachMsg(sender *PeerID, candidates []*Entry) *TeachMsg {
	msg := new(TeachMsg)
	msg.Sender_ = sender
	msg.Announce = candidates
	msg.MsgType = MSG_TEACH
	msg.MsgSize = uint16(4 + sender.Size())
	for _, e := range candidates {
		msg.MsgSize += uint16(e.Size())
	}
	return msg
}

// String returns a human-readable representation of the message
func (m *TeachMsg) String() string {
	return fmt.Sprintf("Teach{%s:%d}", m.Sender_, len(m.Announce))
}
