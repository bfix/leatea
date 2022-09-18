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
	"fmt"
	"leatea/core"
	"leatea/sim"
	"log"
	"strings"
)

type EventHandler struct {
	changed bool
	getID   func(*core.PeerID) int
}

func NewEventHandler(getID func(*core.PeerID) int) *EventHandler {
	return &EventHandler{
		changed: false,
		getID:   getID,
	}
}

func (hdlr *EventHandler) Changed() bool {
	rc := hdlr.changed
	hdlr.changed = false
	return rc
}

func (hdlr *EventHandler) printEntry(f *core.Entry) string {
	return fmt.Sprintf("{%d,%d,%d,%.3f}",
		hdlr.getID(f.Peer), hdlr.getID(f.NextHop),
		f.Hops, f.Origin.Age().Seconds())
}

//nolint:gocyclo // life is complex sometimes...
func (hdlr *EventHandler) HandleEvent(ev *core.Event) {
	// check if event is to be displayed.
	show := false
	for _, t := range sim.Cfg.Options.Events {
		if (t < 0 && -t != ev.Type) || (t == ev.Type) {
			show = true
			break
		}
	}
	if !sim.Cfg.Options.ShowEvents {
		show = !show
	}
	// log network events
	switch ev.Type {

	//------------------------------------------------------------------
	case sim.EvNodeAdded:
		if show {
			val := core.GetVal[[]int](ev)
			log.Printf("[%s] %04X started as #%d (%d running)",
				ev.Peer, ev.Peer.Tag(), val[0], val[1])
		}
		redraw = true

	//------------------------------------------------------------------
	case sim.EvNodeRemoved:
		val := core.GetVal[[]int](ev)
		remain := val[1]
		if remain < 0 {
			remain = netw.StopNodeByID(ev.Peer)
		}
		if show {
			log.Printf("[%s] #%d stopped (%d running)",
				ev.Peer, val[0], remain)
		}
		redraw = true

	//------------------------------------------------------------------
	case core.EvNeighborAdded:
		if show {
			log.Printf("[%d] neighbor #%d added",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvNeighborUpdated:
		if show {
			log.Printf("[%d] neighbor #%d updated",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvNeighborExpired:
		if show {
			log.Printf("[%d] neighbor %d expired",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}
		redraw = true
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvForwardLearned:
		if show {
			e := core.GetVal[*core.Entry](ev)
			log.Printf("[%d < %d] learned %s",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref), hdlr.printEntry(e))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvForwardChanged:
		if show {
			fw := core.GetVal[[3]*core.Entry](ev)
			log.Printf("[%d < %d] %s < %s > %s",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref),
				hdlr.printEntry(fw[0]), hdlr.printEntry(fw[1]), hdlr.printEntry(fw[2]))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvShorterRoute:
		if show {
			log.Printf("[%d] shorter path to %d learned",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvRelayRemoved:
		if show {
			log.Printf("[%d] forward to %d removed",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvRelayRevived:
		if show {
			log.Printf("[%d] revived relay to %d",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvNeighborRelayed:
		if show {
			log.Printf("[%d] revived neighbor as relay to %d",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvLearning:
		if show {
			log.Printf("[%d] learning from %d",
				hdlr.getID(ev.Peer), hdlr.getID(ev.Ref))
		}

	//------------------------------------------------------------------
	case core.EvTeaching:
		if show {
			val := core.GetVal[[]any](ev)
			msg, _ := val[0].(*core.TEAchMsg)
			counts, _ := val[1].([4]int)
			numAnnounce := len(msg.Announce)
			log.Printf("[%d] teaching: %d removed, %d unfiltered, %d pending, %d skipped",
				hdlr.getID(ev.Peer), counts[0], counts[1], counts[2], counts[3]-numAnnounce)
			announced := make([]string, 0)
			for _, ann := range msg.Announce {
				e := &core.Entry{
					Peer:    ann.Peer,
					Hops:    ann.Hops,
					NextHop: msg.Sender(),
					Origin:  core.TimeFromAge(ann.Age),
				}
				announced = append(announced, hdlr.printEntry(e))
			}
			log.Printf("[%d] TEAch [%s]",
				hdlr.getID(ev.Peer), strings.Join(announced, ","))
		}

	//------------------------------------------------------------------
	case core.EvWantToLearn:
		if show {
			log.Printf("[%d] broadcasting LEArn", hdlr.getID(ev.Peer))
		}
	}
}
