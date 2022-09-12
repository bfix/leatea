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
	"sort"
	"sync"
	"time"

	"github.com/bfix/gospel/data"
)

//----------------------------------------------------------------------
// Forwarding table: each peer has a forwarding table with entries for
// all other peers it learned about over time. The entry specifies the
// peer ID of the other peer, the next hop on the route to the target,
// the number of hops to reach the target and a timestamp when the peer
// was last seen in the network. A direct neighbor (within broadcast
// range) has no next hop and a hop count of 0 in the table.
//
// If an entry is removed (because the neighbor expired), the hop
// count is set to -1 to indicated a deleted entry. Once such entry
// is forwarded in a teach message, the entry is removed from the table.
//----------------------------------------------------------------------

// Forward (target peer, distance/hops and age)
// Forwards are send by peers to their neighbors to inform them about
// target peers they know about (see TEAch message handling). If a
// received forward is not in the forward table of a peer, it is added
// with the sender as next hop and a hop count increased by 1. The age
// of the forward is preserved in the new table entry.
type Forward struct {
	// target node
	Peer *PeerID

	// expected number of hops to target
	// (0 = neighbor, -1 = removed neighbor)
	Hops int16 `size:"big"`

	// age of entry since creation (set when sending message)
	Age *Age
}

// Size returns the size of the binary representation (used to calculate
// size of TEAch message based on number of Forward entries)
func (f *Forward) Size() uint {
	var id *PeerID
	var age *Age
	return id.Size() + age.Size() + 2
}

// String returns a human-readable representation
func (f *Forward) String() string {
	return fmt.Sprintf("{%s,%d,%s}", f.Peer, f.Hops, f.Age.String())
}

//......................................................................

// Entry in forward table
type Entry struct {
	Forward

	// Next hop (nil for neighbors)
	NextHop *PeerID

	// Timestamp of the forward (route)
	// It is the time the target was seen by its neighbor from which
	// this route originated.
	Origin *Time

	// Entry changed but not forwarded yet
	// It is set to true of new and changed entries. It flags forwards
	// that the node learned that have not be been send in a TEAch yet.
	Pending bool
}

// Target returns the Forward for a table entry.
// The age of the entry is calculated relative to TimeNow()
func (e *Entry) Target() *Forward {
	return &Forward{
		Peer: e.Peer,
		Hops: e.Hops,
		Age:  e.Origin.Age(),
	}
}

// String returns a human-readable representation
func (e *Entry) String() string {
	e.Age = e.Origin.Age()
	return fmt.Sprintf("{%s,%s}", e.Forward.String(), e.NextHop)
}

//----------------------------------------------------------------------
// FowardTable holds a list of entries (full forwards) to all targets
// learned from the leatea protocol.
//----------------------------------------------------------------------

// ForwardTable is a map of entries with key "target"
type ForwardTable struct {
	sync.RWMutex
	self     *PeerID           // reference to ourself
	recs     map[string]*Entry // forward table as records of entries
	listener Listener          // listener for events

}

// NewForwardTable creates an empty table
func NewForwardTable(self *PeerID) *ForwardTable {
	return &ForwardTable{
		self: self,
		recs: make(map[string]*Entry),
	}
}

// Reset routing table
func (tbl *ForwardTable) Reset() {
	tbl.Lock()
	defer tbl.Unlock()
	tbl.recs = make(map[string]*Entry)
}

// AddNeighbor to forward table:
// A (new) neighbor was seen being active (we received a message from it),
// so the entry for the neighbor is either added to or updated in the table.
func (tbl *ForwardTable) AddNeighbor(node *PeerID) {
	tbl.Lock()
	defer tbl.Unlock()
	// check if entry exists
	if entry, ok := tbl.recs[node.Key()]; ok {
		// exists: update timestamp
		entry.Origin = TimeNow()
		entry.Pending = true
		return
	}
	// new neighbor: insert new entry into table
	tbl.recs[node.Key()] = &Entry{
		Forward: Forward{
			Peer: node,
			Hops: 0,
		},
		NextHop: nil,
		Origin:  TimeNow(),
		Pending: true,
	}
}

// Cleanup forward table and flag expired neighbors (and their dependencies)
// for removal. The actual deletion of the entry in the table happens after
// the removed entry was broadcasted in a TEAch message.
func (tbl *ForwardTable) Cleanup() {
	tbl.RLock()
	defer tbl.RUnlock()

	// remove expired neighbors (and their dependent forwards)
	for _, entry := range tbl.recs {
		// is entry a neighbor?
		if entry.NextHop != nil {
			// no:
			continue
		}
		// is entry pending for deletion?
		if entry.Hops < 0 {
			// yes: already flagged
			continue
		}
		// has the entry expired?
		if !entry.Origin.Expired(time.Duration(cfg.TTLEntry) * time.Second) {
			// no:
			continue
		}
		// notify listener
		if tbl.listener != nil {
			tbl.listener(&Event{
				Type: EvNeighborExpired,
				Peer: tbl.self,
				Ref:  entry.Peer,
			})
		}
		// remove neighbor
		entry.Hops = -2
		entry.NextHop = nil
		entry.Pending = true

		// remove dependent forwards
		for _, fw := range tbl.recs {
			// only forwards where next hop equals neighbor
			if fw.NextHop.Equal(entry.Peer) {
				// remove forward
				fw.Hops = -1
				fw.Origin = entry.Origin
				fw.Pending = true
				// notify listener we removed a forward
				if tbl.listener != nil {
					tbl.listener(&Event{
						Type: EvForwardRemoved,
						Peer: tbl.self,
						Ref:  fw.Peer,
					})
				}
			}
		}
	}
}

// Filter returns a bloomfilter from all table entries (PeerID).
// Remove expired entries first.
func (tbl *ForwardTable) Filter() *data.SaltedBloomFilter {
	// clean-up first
	tbl.Cleanup()

	// create bloomfilter
	tbl.Lock()
	defer tbl.Unlock()
	salt := RndUInt32()
	n := len(tbl.recs) + 2
	fpr := 1. / float64(n)
	pf := data.NewSaltedBloomFilter(salt, n, fpr)

	// add all table entries that are not tagged for deletion
	for _, entry := range tbl.recs {
		// skip removed entry
		if entry.Hops < 0 {
			continue
		}
		// add entry to filter
		pf.Add(entry.Peer.Bytes())
	}
	// add ourself to the filter (can't learn about myself from others)
	pf.Add(tbl.self.Bytes())
	return pf
}

// Candiates returns a list of table entries that are not filtered out by the
// bloomfilter contained in the LEArn message.
// Candiates also can't have sender as next hop. (TODO: check!!)
// Pending entries (updated but not forwarded yet) are collected if there is
// space for them in the result list.
func (tbl *ForwardTable) Candidates(m *LEArnMsg) (list []*Forward) {
	tbl.Lock()
	defer tbl.Unlock()

	// collect forwards for response
	for _, entry := range tbl.recs {
		// add entry if not filtered
		add := !m.Filter.Contains(entry.Peer.Bytes())
		// create forward for response
		forward := entry.Target()
		switch entry.Hops {
		// removed forward
		case -1:
			// forced add
			add = true
		// removed neighbor
		case -2:
			// tag as deleted neighbor
			entry.Hops = -3
			// forced add
			add = true
		case -3:
			// do not add deleted neighbors (even when unfiltered)
			add = false
		}
		// add forward to response if required
		if add {
			list = append(list, forward)
		}
	}
	// honor TEAch limit.
	if len(list) > cfg.MaxTeachs {
		// sort list by ascending number of hops
		sort.Slice(list, func(i, j int) bool {
			return list[i].Hops < list[j].Hops
		})
		list = list[:cfg.MaxTeachs]
	}
	// if we have removed forwards in our response, remove them
	// from the forward table
	for _, forward := range list {
		if forward.Hops == -1 {
			// remove forward from table
			delete(tbl.recs, forward.Peer.Key())
		}
	}
	return
}

// Learn from announcements in a TEAch message
func (tbl *ForwardTable) Learn(m *TEAchMsg) {
	tbl.Lock()
	defer tbl.Unlock()

	// process all announcements
	sender := m.Sender()
	for _, announce := range m.Announce {
		// ignore announcements about ourself
		if announce.Peer.Equal(tbl.self) {
			continue
		}
		// get the timestamp of the announcement
		origin := TimeFromAge(announce.Age)

		// get corresponding forward entry
		key := announce.Peer.Key()
		if entry, ok := tbl.recs[key]; ok {
			// entry exists in the forward table:

			// out-dated announcement?
			if origin.Before(entry.Origin) {
				// yes: ignore old information
				continue
			}
			// check for update
			oldForward := entry.Target()
			changed := false
			if announce.Hops < 0 && entry.Hops >= 0 {
				// "removal" announced
				if announce.Hops == -2 {
					// removed neighbor
					entry.Hops = -2
					entry.NextHop = nil
					entry.Origin = origin
					entry.Pending = true
					changed = true
					// remove dependent forwards
					for _, fw := range tbl.recs {
						if fw.NextHop.Equal(sender) {
							// flag forward for removal
							fw.Hops = -1
							fw.Origin = origin
							fw.Pending = true
							// notify listener we removed a forward
							if tbl.listener != nil {
								tbl.listener(&Event{
									Type: EvForwardRemoved,
									Peer: tbl.self,
									Ref:  fw.Peer,
								})
							}
						}
					}
				} else if entry.NextHop.Equal(sender) {
					// remove dependent forward
					entry.Hops = -1
					entry.Origin = origin
					entry.Pending = true
					// notify listener we removed a forward
					if tbl.listener != nil {
						tbl.listener(&Event{
							Type: EvForwardRemoved,
							Peer: tbl.self,
							Ref:  entry.Peer,
						})
					}
				}
			} else if entry.Hops > announce.Hops+1 {
				// update with shorter path
				entry.Hops = announce.Hops + 1
				entry.NextHop = sender
				entry.Origin = origin
				entry.Pending = true
				changed = true
				// notify listener we removed a forward
				if tbl.listener != nil {
					tbl.listener(&Event{
						Type: EvShorterPath,
						Peer: tbl.self,
						Ref:  entry.Peer,
					})
				}
			}
			// notify listener if table entr has changed
			if changed && tbl.listener != nil {
				tbl.listener(&Event{
					Type: EvForwardTblChanged,
					Peer: tbl.self,
					Ref:  sender,
					Val:  [3]*Forward{oldForward, announce, entry.Target()},
				})
			}
		} else {
			// entry not yet known: add new entry to table
			tbl.recs[key] = &Entry{
				Forward: Forward{
					Peer: announce.Peer,
					Hops: announce.Hops + 1,
				},
				NextHop: sender,
				Origin:  origin,
				Pending: true,
			}
		}
	}
}

// Forward returns the peerid of the next hop to target and the number of
// expected hops along the route.
func (tbl *ForwardTable) Forward(target *PeerID) (*PeerID, int) {
	tbl.RLock()
	defer tbl.RUnlock()
	// lookup entry in table
	if entry, ok := tbl.recs[target.Key()]; ok {
		// ignore removed entries
		if entry.Hops < 0 {
			return nil, 0
		}
		// return forward information
		return entry.NextHop, int(entry.Hops) + 1
	}
	// target not in table
	return nil, 0
}

// NumForwards returns the number of (active) targets in the forward table
func (tbl *ForwardTable) NumForwards() (count int) {
	tbl.RLock()
	defer tbl.RUnlock()
	// count number of active forwards (including neighbors)
	for _, entry := range tbl.recs {
		if entry.Hops >= 0 {
			count++
		}
	}
	return
}

// Return a list of active direct neighbors
func (tbl *ForwardTable) Neighbors() (list []*PeerID) {
	tbl.RLock()
	defer tbl.RUnlock()
	// collect neighbors from the table
	for _, entry := range tbl.recs {
		if entry.NextHop == nil && entry.Hops == 0 {
			list = append(list, entry.Peer)
		}
	}
	return
}
