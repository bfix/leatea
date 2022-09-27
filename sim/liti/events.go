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
	"encoding/binary"
	"fmt"
	"leatea/core"
	"leatea/sim"
	"log"
	"os"
	"strings"
	"sync"
)

type EventHandler struct {
	sync.Mutex

	changed bool
	redraw  bool
	log     *os.File
}

func NewEventHandler() *EventHandler {
	hdlr := &EventHandler{
		changed: false,
		redraw:  false,
	}
	logName := sim.Cfg.Options.EventLog
	if len(logName) > 0 {
		var err error
		if hdlr.log, err = os.Create(logName); err != nil {
			log.Fatal(err)
		}
	}
	return hdlr
}

func (hdlr *EventHandler) Close() {
	if hdlr.log != nil {
		hdlr.log.Close()
	}
}

func (hdlr *EventHandler) State() (changed, redraw bool) {
	hdlr.Lock()
	defer hdlr.Unlock()

	changed = hdlr.changed
	hdlr.changed = false
	redraw = hdlr.redraw
	hdlr.redraw = false
	return
}

func (hdlr *EventHandler) printEntry(f *core.Entry) string {
	return fmt.Sprintf("{%s,%s,%d,%.3f}",
		f.Peer, f.NextHop, f.Hops, f.Origin.Age().Seconds())
}

func (hdlr *EventHandler) printForward(f *core.Forward) string {
	return fmt.Sprintf("{%s,%08X,%d,%.3f}",
		f.Peer, f.NextHop, f.Hops, f.Age.Seconds())
}

//nolint:gocyclo // life is complex sometimes...
func (hdlr *EventHandler) HandleEvent(ev *core.Event) {
	hdlr.Lock()
	defer hdlr.Unlock()

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
			log.Printf("[%s] %08X started as %d (%d running)",
				ev.Peer, ev.Peer.Tag(), val[0], val[1])
		}
		hdlr.redraw = true

	//------------------------------------------------------------------
	case sim.EvNodeRemoved:
		if show {
			val := core.GetVal[[]int](ev)
			log.Printf("[%s] %d stopped (%d running)",
				ev.Peer, val[0], val[1])
		}
		hdlr.redraw = true

	//------------------------------------------------------------------
	case core.EvNeighborAdded:
		if show {
			log.Printf("[%s] neighbor %s added", ev.Peer, ev.Ref)
		}
		hdlr.LogBytes(ev)
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvNeighborUpdated:
		if show {
			log.Printf("[%s] neighbor %s updated", ev.Peer, ev.Ref)
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvNeighborExpired:
		if show {
			log.Printf("[%s] neighbor %s expired", ev.Peer, ev.Ref)
		}
		hdlr.LogBytes(ev)
		hdlr.changed = true
		hdlr.redraw = true

	//------------------------------------------------------------------
	case core.EvForwardLearned:
		if show {
			e := core.GetVal[*core.Entry](ev)
			log.Printf("[%s < %s] learned %s",
				ev.Peer, ev.Ref, hdlr.printEntry(e))
		}
		hdlr.LogBytes(ev)
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvForwardChanged:
		if show {
			val := core.GetVal[[3]*core.Entry](ev)
			log.Printf("[%s < %s] %s < %s > %s",
				ev.Peer, ev.Ref,
				hdlr.printEntry(val[0]), hdlr.printEntry(val[1]), hdlr.printEntry(val[2]))
		}
		hdlr.LogBytes(ev)
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvShorterRoute:
		if show {
			log.Printf("[%s] shorter path to %s learned", ev.Peer, ev.Ref)
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvRelayRemoved:
		if show {
			log.Printf("[%s] forward to %s removed", ev.Peer, ev.Ref)
		}
		hdlr.LogBytes(ev)
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvRelayRevived:
		if show {
			log.Printf("[%s] revived relay to %s", ev.Peer, ev.Ref)
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvNeighborRelayed:
		if show {
			log.Printf("[%s] revived neighbor as relay to %s", ev.Peer, ev.Ref)
		}
		hdlr.changed = true

	//------------------------------------------------------------------
	case core.EvLearning:
		if show {
			log.Printf("[%s] learning from %s", ev.Peer, ev.Ref)
		}

	//------------------------------------------------------------------
	case core.EvTeaching:
		if show {
			val := core.GetVal[[]any](ev)
			msg, _ := val[0].(*core.TEAchMsg)
			counts, _ := val[1].([4]int)
			log.Printf("[%s] teaching: %d removed, %d unfiltered, %d pending, %d skipped",
				ev.Peer, counts[0], counts[1], counts[2], counts[3])
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
			log.Printf("[%s] TEAch [%s]",
				ev.Peer, strings.Join(announced, ","))
		}

	//------------------------------------------------------------------
	case core.EvWantToLearn:
		if show {
			log.Printf("[%s] broadcasting LEArn", ev.Peer)
		}

	//------------------------------------------------------------------
	case core.EvLoopDetect:
		if show {
			val := core.GetVal[[]any](ev)
			entry, _ := val[0].(*core.Entry)
			announce, _ := val[1].(*core.Forward)
			log.Printf("[%s] %s <- [%s] %s",
				ev.Peer, hdlr.printEntry(entry),
				ev.Ref, hdlr.printForward(announce))
		}
	}
}

func (hdlr *EventHandler) writeEntry(e *core.Entry) {
	_, _ = hdlr.log.Write(e.Peer.Data)
	if e.NextHop == nil {
		_, _ = hdlr.log.Write([]byte{0})
	} else {
		_, _ = hdlr.log.Write([]byte{1})
		_, _ = hdlr.log.Write(e.NextHop.Data)
	}
	_ = binary.Write(hdlr.log, binary.BigEndian, e.Hops)
}

func (hdlr *EventHandler) LogBytes(ev *core.Event) {
	if hdlr.log == nil {
		return
	}
	_ = binary.Write(hdlr.log, binary.BigEndian, uint32(ev.Type))
	_ = binary.Write(hdlr.log, binary.BigEndian, ev.Seq)
	_, _ = hdlr.log.Write(ev.Peer.Data)
	_, _ = hdlr.log.Write(ev.Ref.Data)
	switch ev.Type {

	case core.EvForwardChanged:
		val := core.GetVal[[3]*core.Entry](ev)
		hdlr.writeEntry(val[2])

	case core.EvForwardLearned:
		e := core.GetVal[*core.Entry](ev)
		hdlr.writeEntry(e)

	case core.EvNeighborAdded:
		// no more data

	case core.EvNeighborExpired:
		// no more data

	case core.EvRelayRemoved:
		// no more data
	}
}
