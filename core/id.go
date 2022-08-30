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

	"github.com/bfix/gospel/crypto/ed25519"
)

type PeerID struct {
	Data []byte `size:"32"`
	pub  *ed25519.PublicKey
}

func (p *PeerID) Key() string {
	return base64.StdEncoding.EncodeToString(p.Data)
}

func (p *PeerID) Short() string {
	s := base32.StdEncoding.EncodeToString(p.Data)
	return s[:8]
}

func (p *PeerID) Equal(q *PeerID) bool {
	if q == nil && p == nil {
		return true
	}
	if q == nil || p == nil {
		return false
	}
	return bytes.Equal(p.Data, q.Data)
}

func (p *PeerID) Bytes() []byte {
	return Clone(p.Data)
}

type PeerPrivate struct {
	Data []byte `size:"32"`
	prv  *ed25519.PrivateKey
}

func NewPeerPrivate() *PeerPrivate {
	_, prv := ed25519.NewKeypair()
	return &PeerPrivate{
		Data: prv.Bytes(),
		prv:  prv,
	}
}

func (p *PeerPrivate) Public() *PeerID {
	pub := p.prv.Public()
	return &PeerID{
		Data: pub.Bytes(),
		pub:  pub,
	}
}
