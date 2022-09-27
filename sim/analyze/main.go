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

import (
	"encoding/base32"
	"encoding/binary"
	"flag"
	"io"
	"leatea/core"
	"log"
	"os"
)

type LogEntry struct {
	Type    uint32
	Peer    [32]byte
	Ref     [32]byte
	Target  [32]byte
	NextHop [32]byte
	Hops    uint32
}

func (e *LogEntry) WithForward() bool {
	return e.Type == core.EvForwardChanged
}

type Forward struct {
	next string
	hops int16
}

type Node struct {
	self     string
	forwards map[string]*Forward
}

func NewNode(self string) *Node {
	node := new(Node)
	node.self = self
	node.forwards = make(map[string]*Forward)
	return node
}

func (n *Node) SetForward(target, next string, hops int16) {
	forward, ok := n.forwards[target]
	if !ok {
		forward = new(Forward)
		n.forwards[target] = forward
	}
	forward.next = next
	forward.hops = hops
}

var (
	index = make(map[string]*Node)
	seq   = make(map[string]uint32)
)

// run application
func main() {
	log.Println("LEArn/TEAch routing analyzer")
	log.Println("(c) 2022, Bernd Fix     >Y<")

	// parse arguments
	var eventLog string
	flag.StringVar(&eventLog, "i", "", "event log (binary)")
	flag.Parse()

	// read event log and reconstruct forward tables of node step by step
	f, err := os.Open(eventLog)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	ev := new(LogEntry)
	flag := make([]byte, 1)
	for k := 1; ; k++ {
		// read and handle next log entry
		if err := binary.Read(f, binary.BigEndian, &ev.Type); err != nil {
			if err == io.EOF {
				log.Printf("%d log entries read.", k-1)
				break
			}
			log.Fatal(err)
		}
		var inSeq uint32
		_ = binary.Read(f, binary.BigEndian, &inSeq)

		_, _ = f.Read(ev.Peer[:])
		self := base32.StdEncoding.EncodeToString(ev.Peer[:5])[:8]
		lastSeq, ok := seq[self]
		if ok && lastSeq > inSeq {
			log.Fatalf("*** Sequence error in log entry #%d", k)
		}
		seq[self] = inSeq
		node, ok := index[self]
		if !ok {
			node = NewNode(self)
			index[self] = node
		}
		_, _ = f.Read(ev.Ref[:])
		ref := base32.StdEncoding.EncodeToString(ev.Ref[:5])[:8]

		//log.Printf("[%s < %s] Seq=%d, Type=%d", self, ref, inSeq, ev.Type)

		switch ev.Type {
		case core.EvForwardChanged, core.EvForwardLearned:
			_, _ = f.Read(ev.Target[:])
			_, _ = f.Read(flag)
			next := ""
			if flag[0] == 1 {
				_, _ = f.Read(ev.NextHop[:])
				next = base32.StdEncoding.EncodeToString(ev.NextHop[:5])[:8]
			}
			var hops int16
			_ = binary.Read(f, binary.BigEndian, &hops)
			tgt := base32.StdEncoding.EncodeToString(ev.Target[:5])[:8]
			node.SetForward(tgt, next, hops)

		case core.EvNeighborAdded:
			node.SetForward(ref, "", 0)

		case core.EvNeighborExpired, core.EvRelayRemoved:
			node.SetForward(ref, "", -1)
			delete(index, ref)
		}
	}

	// run analysis
	analyzeRoutes()
}
