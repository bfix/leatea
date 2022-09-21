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

// Debugging switch
const Debug = true

// Kind and state of entry / forward
const (
	KindUnknown  = 0
	KindRelay    = 1
	KindNeighbor = 2

	StateInvalid = 0
	StateActive  = 1
	StateRemoved = 2
	StateDormant = 3
)

//----------------------------------------------------------------------
// Forwarding table: each peer has a forwarding table with entries for
// all other peers it has ever learned over time. The entry specifies
// the peer ID of the target, the next hop on the route to the target,
// the number of hops to reach the target and a timestamp when the
// originating entry was created.
//
// There are two kinds of entries: relays and neighbors. An active
// neighbor (within broadcast range) has no next hop and a hop count
// of 0. An active relay has a next hop and a hop count > 0.
//
// Entries can be in different states: active (see above), removed
// and dormant. A removed entry can be included in a TEAch message
// (see Candidates method); it is set to dormant once broadcasted.
// A dormant entry is never taught, but can be revived if a newer
// announcement is received (see Learn method). Removed or dormant
// entries are not considered when forwarding messages.
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
	NextHop uint32 `size:"big"`

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

// Kind of forward
func (f *Forward) Kind() (kind int) {
	switch f.Hops {
	case 0, -2, -4:
		if f.NextHop != 0 {
			kind = KindUnknown
			panic("")
		} else {
			kind = KindNeighbor
		}
	default:
		if f.NextHop == 0 {
			kind = KindUnknown
			panic("")
		} else {
			kind = KindRelay
		}
	}
	if Debug && kind == KindUnknown {
		panic(fmt.Sprintf("unknown kind: %s", f))
	}
	return
}

// State of the forward
func (f *Forward) State() (state int) {
	switch f.Hops {
	case -1, -2:
		state = StateRemoved
	default:
		if f.Hops >= 0 {
			state = StateActive
		} else {
			state = StateInvalid
		}
	}
	if Debug && state == StateInvalid {
		panic(fmt.Sprintf("invalid state: %s", f))
	}
	return
}

// IsA checks if a forward is of given kind and state
func (f *Forward) IsA(kind, state int) bool {
	return f.Kind() == kind && f.State() == state
}

// String returns a human-readable representation
func (f *Forward) String() string {
	if f == nil {
		return "{nil forward}"
	}
	return fmt.Sprintf("{%s,%d,%08X,%.3f}", f.Peer, f.Hops, f.NextHop, f.Age.Seconds())
}

//----------------------------------------------------------------------

// Entry in forward table
type Entry struct {
	// Target node
	Peer *PeerID

	// Expected number of hops to target:
	// The hop count is also used to indicate the status
	// of an entry:
	//   active entries:
	//     >  0: active relay
	//     =  0: active neighbor
	//   removed entries:
	//     = -1: removed relay
	//     = -2: removed neighbor
	//   dormant entries:
	//     = -3: dormant relay
	//     = -4: dormant neighbor
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
	hops := f.Hops
	if f.State() == StateActive {
		hops++
	}
	return &Entry{
		Peer:    f.Peer,
		NextHop: sender,
		Hops:    hops,
		Origin:  TimeFromAge(f.Age),
	}
}

// Target returns the Forward for a table entry.
// The age of the entry is calculated from Origin relative to TimeNow()
func (e *Entry) Target() *Forward {
	return &Forward{
		Peer:    e.Peer.Clone(),
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

// Kind of forward
func (e *Entry) Kind() (kind int) {
	switch e.Hops {
	case 0, -2, -4:
		if e.NextHop != nil {
			kind = KindUnknown
		} else {
			kind = KindNeighbor
		}
	default:
		if e.NextHop == nil {
			kind = KindUnknown
		} else {
			kind = KindRelay
		}
	}
	if Debug && kind == KindUnknown {
		panic(fmt.Sprintf("unknown kind: %s", e))
	}
	return
}

// State of the forward
func (e *Entry) State() (state int) {
	switch e.Hops {
	case -3, -4:
		state = StateDormant
	case -1, -2:
		state = StateRemoved
	default:
		if e.Hops >= 0 {
			state = StateActive
		} else {
			state = StateInvalid
		}
	}
	if Debug && state == StateInvalid {
		panic(fmt.Sprintf("invalid state: %s", e))
	}
	return
}

// Set state of entry
func (e *Entry) SetState(state int) {
	now := TimeNow()
	switch e.Kind() {
	case KindNeighbor:
		switch state {
		case StateActive:
			e.Hops = 0
			e.Origin = now
		case StateRemoved:
			e.Hops = -2
			e.Origin = now
		case StateDormant:
			e.Hops = -4
		default:
			panic("invalid state for neighbor")
		}
		e.NextHop = nil
	case KindRelay:
		switch state {
		case StateRemoved:
			e.Hops = -1
			e.Origin = now
		case StateDormant:
			e.Hops = -3
		default:
			panic("invalid state for relay")
		}
	default:
		panic("unknown kind for state change")
	}
	e.Changed = now
}

// IsA checks if a forward is of given kind and state
func (e *Entry) IsA(kind, state int) bool {
	return e.Kind() == kind && e.State() == state
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
// FowardTable holds a list of entries to all targets learned from the
// leatea protocol:
// Entries, once added to the table, are never removed from the table
// again. If a forward is "removed", it is flagged by hop count (-1 for
// removed relay and -2 for removed neighbor). A removed entry can be
// included in a TEAch message; it is set to "dormant" once it was
// broadcasted (not included in LEArn filters or TEAches).
// Dormant entries can be resurrected by announces; neighbors get
// resurrected when a message from them is received and relays get
// resurrected when a newer relay is learned.
//----------------------------------------------------------------------

// ForwardTable is a map of entries with key "target"
type ForwardTable struct {
	sync.Mutex

	// reference to ourself
	self *PeerID

	// forward table as records of entries
	recs map[string]*Entry

	// listener for events
	listener Listener

	// sanity checker (optional)
	check func(string, ...any)
}

// NewForwardTable creates an empty table
func NewForwardTable(self *PeerID, debug bool) *ForwardTable {
	tbl := &ForwardTable{
		self:  self,
		recs:  make(map[string]*Entry),
		check: nil,
	}
	if debug {
		tbl.check = tbl.sanityCheck
	}
	return tbl
}

//======================================================================
// LEArn / TEAch and beacon message handling
//======================================================================

// Teach about our local forward table
func (tbl *ForwardTable) Teach(msg *LEArnMsg) (*TEAchMsg, [4]int) {
	// build a list of candidate entries for teaching:
	// candidates are not included in the learn filter
	// and don't have the learner as next hop.
	candidates, counts := tbl.candidates(msg)
	if len(candidates) == 0 {
		return nil, counts
	}
	// assemble TEACH message
	return NewTEAchMsg(tbl.self, candidates), counts
}

// AddNeighbor to forward table:
// A (new) neighbor was seen being active (we received a message from it),
// so the entry for the neighbor is either added to or updated in the table.
func (tbl *ForwardTable) AddNeighbor(node *PeerID) {
	tbl.Lock()
	defer func() {
		if Debug {
			tbl.check("add neighbor")
		}
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
}

// Learn from announcements in a TEAch message
func (tbl *ForwardTable) Learn(msg *TEAchMsg) {
	tbl.Lock()
	defer func() {
		if Debug {
			tbl.check("learn", msg.Sender(), msg.Announce)
		}
		tbl.Unlock()
	}()

	// process all announcements
	sender := msg.Sender()
	now := TimeNow()
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

			// handle removed announcements
			hops := announce.Hops + 1
			next := sender
			if announce.IsA(KindRelay, StateRemoved) {
				continue
			} else if announce.IsA(KindNeighbor, StateRemoved) {
				hops = -2
				next = nil
			}
			// create new entry
			e := &Entry{
				Peer:    announce.Peer,
				Hops:    hops,
				NextHop: next,
				Origin:  origin,
				Changed: now,
				Pending: true,
			}
			// add entry to forward table
			tbl.recs[key] = e

			// notify listener
			if tbl.listener != nil {
				tbl.listener(&Event{
					Type: EvForwardLearned,
					Peer: tbl.self,
					Ref:  sender,
					Val:  e,
				})
			}
			continue
		}
		//--------------------------------------------------------------
		// entry exists in the forward table:

		// do not re-learn a removed entry; wait for it to be dormant
		if entry.State() == StateRemoved {
			continue
		}
		// out-dated announcement?
		dt := origin.Diff(entry.Origin)
		if dt < 1 {
			// yes: ignore old information
			continue
		}

		// candidate for update: remove pending flag
		entry.Pending = false

		// remember old entry
		oldEntry := entry.Clone()
		changed := false

		//--------------------------------------------------------------
		// "removal" announced?
		//--------------------------------------------------------------
		if announce.State() == StateRemoved {
			// yes: continue if entry is already removed or dormant
			if entry.State() != StateActive {
				continue
			}
			// neighbor entry?
			if entry.Kind() == KindNeighbor {
				// broadcast entry to counter the removal
				entry.Pending = true
				log.Printf("[%s] sender %s: announce = %s,entry = %s", tbl.self, sender, announce, entry)
				panic("1") // continue
			}
			// relay entry:

			// (t,sender,...) <- sender->(t,...)
			if entry.NextHop.Equal(sender) {
				// remove relay
				entry.SetState(StateRemoved)
				entry.Origin = origin
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

			log.Printf("[%s] sender %s: announce = %s,entry = %s", tbl.self, sender, announce, entry)
			panic("2")

		} else if entry.Kind() == KindRelay {
			// relay:

			// only update on dormant entry or shorter route
			evType := 0
			switch {
			case announce.Hops+1 < entry.Hops:
				evType = EvShorterRoute
			case announce.Hops+1 == entry.Hops && !sender.Equal(entry.NextHop):
				evType = EvRelayUpdated
			case entry.State() == StateDormant:
				evType = EvRelayRevived
			default:
				continue
			}
			// possible loop construction?
			if entry.NextHop.Equal(sender) && announce.NextHop == tbl.self.Tag() {
				log.Printf("LOOP? local %s = %s, remote %s = %s",
					tbl.self, entry, sender, announce)
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
					Type: evType,
					Peer: tbl.self,
					Ref:  entry.Peer,
				})
			}
		} else if entry.IsA(KindNeighbor, StateDormant) {
			// dormant neighbor:

			// update with newer relay
			entry.Hops = announce.Hops + 1
			entry.NextHop = sender
			entry.Origin = origin
			entry.Changed = now
			entry.Pending = true
			changed = true

			// notify listener if a shorter route was found
			if tbl.listener != nil {
				tbl.listener(&Event{
					Type: EvNeighborRelayed,
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

//======================================================================
// Helper methods for message handling
//======================================================================

// cleanup forward table and flag expired neighbors (and their dependencies)
// for removal. The actual deletion of the entry in the table happens after
// the removed entry was broadcasted in a TEAch message.
func (tbl *ForwardTable) cleanup() {
	tbl.Lock()
	defer func() {
		if Debug {
			tbl.check("clean-up")
		}
		tbl.Unlock()
	}()

	// remove expired neighbors (and their dependent relays)
	for _, entry := range tbl.recs {
		// is entry an active neighbor?
		if !entry.IsA(KindNeighbor, StateActive) {
			// no:
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
		// remove neighbor
		entry.SetState(StateRemoved)
		entry.Pending = true

		// remove dependent relays
		for _, fw := range tbl.recs {
			// only relays where next hop equals neighbor
			if fw.NextHop.Equal(entry.Peer) {
				// remove forward
				fw.SetState(StateRemoved)
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

// filter returns a bloomfilter from all table entries (PeerID).
// Remove expired entries first.
func (tbl *ForwardTable) filter() *data.SaltedBloomFilter {
	// clean-up first
	tbl.cleanup()

	// create bloomfilter
	tbl.Lock()
	defer tbl.Unlock()
	salt := RndUInt32()
	n := len(tbl.recs) + 2
	fpr := 1. / float64(n)
	pf := data.NewSaltedBloomFilter(salt, n, fpr)

	// process all table entries
	for _, entry := range tbl.recs {
		// skip dormant entries
		if entry.State() == StateDormant {
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
type candidate struct {
	e    *Entry // reference to entry
	kind int    // entry classification (lower value = higher priority)
}

// Candiates returns a list of table entries that are not filtered out by the
// bloomfilter contained in the LEArn message.
// Pending entries (updated but not forwarded yet) are collected if there is
// space for them in the result list.
func (tbl *ForwardTable) candidates(m *LEArnMsg) (list []*Forward, counts [4]int) {
	tbl.Lock()
	defer func() {
		if Debug {
			tbl.check("candidates")
		}
		tbl.Unlock()
	}()

	// collect forwards for response
	collect := make([]*candidate, 0)
	for _, entry := range tbl.recs {
		// new candidate and flag for inclusion
		cnd := &candidate{entry, -1}
		add := false

		// add entry if not filtered
		if !m.Filter.Contains(entry.Peer.Bytes()) {
			add = true
			cnd.kind = 0 // unfiltered entry
		}
		// don't add dormant entries
		if entry.State() == StateDormant {
			add = false
		} else if entry.State() == StateRemoved {
			add = true
			cnd.kind = 1
			if entry.Kind() == KindRelay {
				cnd.kind = 2
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
	counts[3] = 0
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
		counts[3] = len(collect) - cfg.MaxTeachs
		collect = collect[:cfg.MaxTeachs]
	}
	// if we have removed relays in our response, remove them
	// from the forward table. Reset pending flag on entry and
	// correct for removed meighbors (they are zombified).
	for _, cnd := range collect {
		entry := cnd.e
		forward := entry.Target()
		if entry.State() == StateRemoved {
			// tag entry as dormant
			entry.SetState(StateDormant)
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

//======================================================================
// Public access methods
//======================================================================

// Forward returns the peerid of the next hop to target and the number of
// expected hops along the route.
func (tbl *ForwardTable) Forward(target *PeerID) (*PeerID, int) {
	tbl.Lock()
	defer tbl.Unlock()
	// lookup entry in table
	if entry, ok := tbl.recs[target.Key()]; ok {
		// ignore removed or dormant entries
		if entry.Hops < 0 {
			return nil, 0
		}
		// return forward information
		return entry.NextHop.Clone(), int(entry.Hops) + 1
	}
	// target not in table
	return nil, 0
}

// NumForwards returns the number of (active) targets in the forward table
func (tbl *ForwardTable) NumForwards() (count int) {
	tbl.Lock()
	defer tbl.Unlock()
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
	tbl.Lock()
	defer tbl.Unlock()
	for _, entry := range tbl.recs {
		list = append(list, entry.Clone())
	}
	return
}

// Return a list of active direct neighbors
func (tbl *ForwardTable) Neighbors() (list []*PeerID) {
	tbl.Lock()
	defer tbl.Unlock()
	// collect neighbors from the table
	for _, entry := range tbl.recs {
		if entry.IsA(KindNeighbor, StateActive) {
			list = append(list, entry.Peer.Clone())
		}
	}
	return
}

//======================================================================
// Debug helpers
//======================================================================

// sanity check of forward table in debug mode.
// (only call from within a locked table instance!)
func (tbl *ForwardTable) sanityCheck(label string, args ...any) {
	// check all forward entries in table
	for _, entry := range tbl.recs {

		// check for valid target
		if entry.Peer == nil {
			log.Printf("[%s] peer %s forward to nil", label, tbl.self)
			panic(label)
		}
		// check for self target
		if entry.Peer.Equal(tbl.self) {
			log.Printf("[%s] peer %s forward to self", label, tbl.self)
			panic(label)
		}
		// check entry
		if entry.Kind() == KindRelay {
			// relay:

			// check for valid neighor as next hop
			nb, ok := tbl.recs[entry.NextHop.Key()]
			if !ok {
				log.Printf("[%s] peer %s has forward %s with unknown next hop", label, tbl.self, entry.Peer)
				for i, arg := range args {
					log.Printf("Arg #%d: %v", i+1, arg)
				}
				log.Printf("Bad entry: %s", entry)
				panic(label)
			}
			if nb.Kind() != KindNeighbor {
				log.Printf("[%s] peer %s has forward %s with invalid next hop", label, tbl.self, entry.Peer)
				for i, arg := range args {
					log.Printf("Arg #%d: %v", i+1, arg)
				}
				log.Printf("Bad entry: %s / %s", entry, nb)
				panic(label)
			}
		}
	}
}
