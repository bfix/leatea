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
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"

	"github.com/bfix/gospel/crypto/ed25519"
)

//----------------------------------------------------------------------

// PeerID is the identifier for a node in the network. It is the binary
// representation of the public Ed25519 key of a node.
type PeerID struct {
	Data []byte `size:"(Size)" init:"Init"` // binary representation

	// transient
	pub   *ed25519.PublicKey // Ed25519 pubkey
	tag   uint32             // short identifier
	str32 string             // string representation (base32)
	str64 string             // string representation (base64)
}

// Create a new PeerID from binary data
func NewPeerID(data []byte) *PeerID {
	p := new(PeerID)
	p.Data = make([]byte, p.Size())
	copy(p.Data, data)
	p.Init()
	return p
}

// Initialize transient attributes based on Data
func (p *PeerID) Init() {
	if p != nil {
		p.tag = binary.BigEndian.Uint32(p.Data[:4])
		p.str64 = base64.StdEncoding.EncodeToString(p.Data)
		p.str32 = base32.StdEncoding.EncodeToString(p.Data)[:8]
		if p.pub == nil {
			p.pub = ed25519.NewPublicKeyFromBytes(p.Data)
		}
	}
}

// Size of a peerid (used for serialization).
func (p *PeerID) Size() uint {
	return 32
}

// Tag (short identifier) of the peer id
func (p *PeerID) Tag() uint32 {
	if p == nil {
		return 0
	}
	return p.tag
}

// Key returns a string used for map operations
func (p *PeerID) Key() string {
	if p == nil {
		return ""
	}
	return p.str64
}

// String returns a human-readable short peer identifier
func (p *PeerID) String() string {
	if p == nil {
		return "(none)"
	}
	return p.str32
}

// Equal returns true if two peerids are equal
func (p *PeerID) Equal(q *PeerID) bool {
	// handle edge cases involving nil pointers
	if q == nil && p == nil {
		return true
	}
	if q == nil || p == nil {
		return false
	}
	// compare binary representations
	return bytes.Equal(p.Data, q.Data)
}

// Bytes returns the binary representation (as a clone)
func (p *PeerID) Bytes() []byte {
	return Clone(p.Data)
}

//----------------------------------------------------------------------

// PeerPrivate is the binary representation of the long-term signing key
// of the node (a Ed25519 private key)
type PeerPrivate struct {
	Data []byte `size:"(Size)"` // binary representation

	// transient
	prv *ed25519.PrivateKey // node private signng key
}

// NewPeerPrivate creates a new node private signing key
func NewPeerPrivate() *PeerPrivate {
	_, prv := ed25519.NewKeypair()
	return &PeerPrivate{
		Data: prv.Bytes(),
		prv:  prv,
	}
}

// Size of a peer private key (used for local serialization).
func (p *PeerPrivate) Size() uint {
	return 64
}

// Public returns the peerid (binary representation of the public Ed25519 key
// of the node)
func (p *PeerPrivate) Public() *PeerID {
	pub := p.prv.Public()
	id := &PeerID{
		Data: pub.Bytes(),
		pub:  pub,
	}
	id.Init()
	return id
}
