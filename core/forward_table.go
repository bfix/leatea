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
	"log"
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
	// Target node
	Peer *PeerID

	// Expected number of hops to target
	Hops int16 `size:"big"`

	// Short identifier for next hop
	NextHop uint16 `size:"big"`

	// Age of entry since creation of the originating entry
	Age Age
}

// Size returns the size of the binary representation (used to calculate
// size of TEAch message based on number of Forward entries)
func (f *Forward) Size() uint {
	var id *PeerID
	var age Age
	return id.Size() + age.Size() + 4
}

// String returns a human-readable representation
func (f *Forward) String() string {
	if f == nil {
		return "{nil forward}"
	}
	return fmt.Sprintf("{%s,%d,%04X,%.3f}", f.Peer, f.Hops, f.NextHop, f.Age.Seconds())
}

//----------------------------------------------------------------------

// Entry in forward table
type Entry struct {
	// Target node
	Peer *PeerID

	// Expected number of hops to target:
	// The hop count is also used to indicate the status
	// of an entry:
	//    >  0: active relay
	//    =  0: active neighbor
	//    = -1: removed relay
	//    = -2: removed neighbor
	//    = -3: zombie neighbor
	Hops int16 `size:"big"`

	// Next hop (nil for neighbors)
	NextHop *PeerID

	// Timestamp of the forward (route)
	// It is the time the target was seen by its neighbor from which
	// this route originated.
	Origin Time

	// Timestamp when the entry was learned/added/updated
	Changed Time

	// Entry changed but not forwarded yet:
	// It is set to true of new and changed entries. It flags forwards
	// that the node learned that have not be been send in a TEAch yet.
	Pending bool
}

// EntryFromForward creates a new Entry from a forward send by sender.
func EntryFromForward(f *Forward, sender *PeerID) *Entry {
	return &Entry{
		Peer:    f.Peer,
		NextHop: sender,
		Hops:    f.Hops + 1,
		Origin:  TimeFromAge(f.Age),
	}
}

// Target returns the Forward for a table entry.
// The age of the entry is calculated from Origin relative to TimeNow()
func (e *Entry) Target() *Forward {
	return &Forward{
		Peer:    e.Peer,
		Hops:    e.Hops,
		NextHop: e.NextHop.Tag(),
		Age:     e.Origin.Age(),
	}
}

// Clone an entry
func (e *Entry) Clone() *Entry {
	return &Entry{
		Peer:    e.Peer,
		Hops:    e.Hops,
		NextHop: e.NextHop,
		Origin:  e.Origin,
		Changed: e.Changed,
		Pending: e.Pending,
	}
}

// String returns a human-readable representation
func (e *Entry) String() string {
	if e == nil {
		return "{nil entry}"
	}
	return fmt.Sprintf("{%s,%s,%d,%.3f}",
		e.Peer, e.NextHop, e.Hops, e.Origin.Age().Seconds())
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
	defer func() {
		tbl.sanityCheck("add neighbor")
		tbl.Unlock()
	}()

	// check if entry exists
	now := TimeNow()
	if entry, ok := tbl.recs[node.Key()]; ok {
		// entry exists: update entry
		// next hop and hop count need to be reset in case
		// the old entry was a relay.
		entry.NextHop = nil
		entry.Hops = 0
		entry.Origin = now
		entry.Changed = now

		// notify listener
		if tbl.listener != nil {
			tbl.listener(&Event{
				Type: EvNeighborUpdated,
				Peer: tbl.self,
				Ref:  node,
			})
		}
		return
	}
	// new neighbor: insert new entry into table
	tbl.recs[node.Key()] = &Entry{
		Peer:    node,
		Hops:    0,
		NextHop: nil,
		Origin:  now,
		Changed: now,
		Pending: true,
	}
	// notify listener
	if tbl.listener != nil {
		tbl.listener(&Event{
			Type: EvNeighborAdded,
			Peer: tbl.self,
			Ref:  node,
		})
	}
	// no dependent relays can exist.
}

// Cleanup forward table and flag expired neighbors (and their dependencies)
// for removal. The actual deletion of the entry in the table happens after
// the removed entry was broadcasted in a TEAch message.
func (tbl *ForwardTable) Cleanup() {
	tbl.RLock()
	defer func() {
		tbl.sanityCheck("clean-up")
		tbl.RUnlock()
	}()

	// remove expired neighbors (and their dependent relays)
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
		// has the neighbor expired?
		if !entry.Origin.Expired(time.Duration(cfg.TTLBeacon) * time.Second) {
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
		now := TimeNow()

		// remove neighbor
		entry.Hops = -2
		entry.Origin = now
		entry.Changed = now
		entry.Pending = true

		// remove dependent relays
		for _, fw := range tbl.recs {
			// only relays where next hop equals neighbor
			if fw.NextHop.Equal(entry.Peer) {
				// remove forward
				fw.Hops = -1
				fw.Origin = now
				fw.Changed = now
				fw.Pending = true
				// notify listener we removed a forward
				if tbl.listener != nil {
					tbl.listener(&Event{
						Type: EvRelayRemoved,
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
		// skip removed relay
		if entry.Hops == -1 {
			continue
		}
		// add entry to filter
		pf.Add(entry.Peer.Bytes())
	}
	// add ourself to the filter (can't learn about myself from others)
	pf.Add(tbl.self.Bytes())
	return pf
}

//----------------------------------------------------------------------

// Candidate entry for inclusion in a TEAch message
type _Candidate struct {
	e    *Entry // reference to entry
	kind int    // entry classification (lower value = higher priority)
}

// Candiates returns a list of table entries that are not filtered out by the
// bloomfilter contained in the LEArn message.
// Pending entries (updated but not forwarded yet) are collected if there is
// space for them in the result list.
func (tbl *ForwardTable) Candidates(m *LEArnMsg) (list []*Forward, counts [4]int) {
	tbl.Lock()
	defer func() {
		tbl.sanityCheck("candidates")
		tbl.Unlock()
	}()
	// collect forwards for response
	collect := make([]*_Candidate, 0)
	for _, entry := range tbl.recs {
		// new candidate and flag for inclusion
		cnd := &_Candidate{entry, -1}
		add := false

		// add entry if not filtered
		if !m.Filter.Contains(entry.Peer.Bytes()) {
			add = true
			cnd.kind = 0 // unfiltered entry
		}
		// handle removed entries
		if entry.Hops < 0 {
			switch entry.Hops {
			case -1:
				// removed relay
				add = true
				cnd.kind = 2
			case -2:
				// removed neighbor
				add = true
				cnd.kind = 1
			case -3:
				// zombie neighbor
				add = false
			}
		} else if entry.Pending {
			// pending entry
			add = true
			cnd.kind = 3
		}
		// add forward to response if required
		if add {
			collect = append(collect, cnd)
		}
	}
	// honor TEAch limit.
	counts[3] = len(collect)
	if counts[3] > cfg.MaxTeachs {
		// sort list by descending kind (primary) and ascending number
		// of hops (secondary)
		sort.Slice(collect, func(i, j int) bool {
			ci := collect[i]
			cj := collect[j]
			if ci.kind < cj.kind {
				return true
			} else if ci.kind > cj.kind {
				return false
			}
			return ci.e.Hops < cj.e.Hops
		})
		// trim list to max. length
		collect = collect[:cfg.MaxTeachs]
	}
	// if we have removed relays in our response, remove them
	// from the forward table. Reset pending flag on entry and
	// correct for removed meighbors (they are zombified).
	for _, cnd := range collect {
		entry := cnd.e
		forward := entry.Target()
		if entry.Hops == -1 {
			// remove relay from table
			delete(tbl.recs, entry.Peer.Key())
			counts[0]++
		} else if entry.Hops == -2 {
			// tag neighbor as zombie
			entry.Hops = -3
			counts[0]++
		} else if entry.Pending {
			counts[2]++
		} else {
			counts[1]++
		}
		// no need to broadcast entry again
		entry.Pending = false
		// add forward to candidates list
		list = append(list, forward)
	}
	return
}

// Learn from announcements in a TEAch message
func (tbl *ForwardTable) Learn(msg *TEAchMsg) {
	tbl.Lock()
	sender := msg.Sender()
	now := TimeNow()
	defer func() {
		tbl.sanityCheck("learn", sender, msg.Announce)
		tbl.Unlock()
	}()

	// process all announcements
	for _, announce := range msg.Announce {
		// ignore announcements about ourself
		if announce.Peer.Equal(tbl.self) {
			continue
		}
		// get the timestamp of the announcement
		origin := TimeFromAge(announce.Age)

		// get corresponding forward entry
		key := announce.Peer.Key()
		entry, ok := tbl.recs[key]
		if !ok {
			//----------------------------------------------------------
			// no entry found:

			// skip removed relay announcements
			if announce.Hops == -1 {
				continue
			}
			// create new entry
			e := &Entry{
				Peer:    announce.Peer,
				Hops:    announce.Hops + 1,
				NextHop: sender,
				Origin:  origin,
				Changed: now,
				Pending: true,
			}
			// correct hops count for removed neighbors
			if announce.Hops == -2 {
				e.Hops = -2
			}
			// add entry to forward table
			tbl.recs[key] = e

			// notify listener
			tbl.listener(&Event{
				Type: EvForwardLearned,
				Peer: tbl.self,
				Ref:  sender,
				Val:  e,
			})
			return
		}
		//--------------------------------------------------------------
		// entry exists in the forward table:

		// out-dated announcement?
		if origin.Before(entry.Origin) {
			// yes: ignore old information
			continue
		}
		// remember old entry
		oldEntry := entry.Clone()
		changed := false

		// "removal" announced?
		if announce.Hops < 0 {
			// yes: continue if entry is already removed
			if entry.Hops < 0 {
				continue
			}
			// remove dependent relay
			if entry.NextHop.Equal(sender) {
				// remove relay
				entry.Hops = -1
				entry.Origin = origin
				entry.Changed = now
				entry.Pending = true
				changed = true

				// notify listener we removed a forward
				if tbl.listener != nil {
					tbl.listener(&Event{
						Type: EvRelayRemoved,
						Peer: tbl.self,
						Ref:  entry.Peer,
					})
				}
			}
		} else if entry.NextHop != nil && announce.Hops+1 < entry.Hops {
			// possible loop construction?
			if entry.NextHop.Equal(sender) && announce.NextHop == tbl.self.Tag() {
				log.Printf("LOOP? local %s = %s, remote %s = %s",
					tbl.self, entry.String(), sender, announce.String())
				continue
			}
			// update relay with newer relay
			entry.Hops = announce.Hops + 1
			entry.NextHop = sender
			entry.Origin = origin
			entry.Changed = now
			entry.Pending = true
			changed = true

			// notify listener if a shorter route was found
			if tbl.listener != nil {
				tbl.listener(&Event{
					Type: EvShorterRoute,
					Peer: tbl.self,
					Ref:  entry.Peer,
				})
			}
		}
		// notify listener if table entry has changed
		if changed && tbl.listener != nil {
			// send event
			annEntry := EntryFromForward(announce, sender)
			tbl.listener(&Event{
				Type: EvForwardChanged,
				Peer: tbl.self,
				Ref:  sender,
				Val:  [3]*Entry{oldEntry, annEntry, entry},
			})
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

// Forwards returns the forward table as list of forward entries.
func (tbl *ForwardTable) Forwards() (list []*Entry) {
	tbl.RLock()
	defer tbl.RUnlock()
	for _, entry := range tbl.recs {
		list = append(list, entry.Clone())
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

func (tbl *ForwardTable) sanityCheck(label string, args ...any) {
	// sanity check: make sure all relays have a valid neighbor as next hop
	for _, entry := range tbl.recs {
		if entry.Peer == nil {
			log.Printf("[%s] peer %s forward to nil", label, tbl.self)
			panic(label)
		}
		if entry.Peer.Equal(tbl.self) {
			log.Printf("[%s] peer %s forward to self", label, tbl.self)
			panic(label)
		}
		if entry.NextHop != nil {
			nb, ok := tbl.recs[entry.NextHop.Key()]
			if !ok {
				log.Printf("[%s] peer %s has forward with unknown next hop", label, tbl.self)
				for i, arg := range args {
					log.Printf("Arg #%d: %v", i+1, arg)
				}
				log.Printf("Bad entry: %s", entry)
				panic(label)
			}
			if nb.NextHop != nil {
				log.Printf("[%s] peer %s has forward with invalid next hop", label, tbl.self)
				for i, arg := range args {
					log.Printf("Arg #%d: %v", i+1, arg)
				}
				log.Printf("Bad entry: %s / %s", entry, nb)
				panic(label)
			}
		}
	}
}
