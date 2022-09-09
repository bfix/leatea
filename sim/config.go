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

package sim

import (
	"encoding/json"
	"leatea/core"
	"math/rand"
	"os"
)

// Random generator (deterministic) for reproducible tests
var Random = rand.New(rand.NewSource(19031962)) //nolint:gosec // deterministic testing

// Wall definition in environment
type WallDef struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
	F  float64 `json:"f"`
}

// EnvironCfg holds configuration data for the environment
type EnvironCfg struct {
	Class    string     `json:"class"`
	Width    float64    `json:"width"`
	Height   float64    `json:"height"`
	NumNodes int        `json:"numNodes"`
	CoolDown int        `json:"cooldown"`
	Walls    []*WallDef `json:"walls"`
}

// NodeCfg holds configuration data for simulated nodes
type NodeCfg struct {
	Reach2     float64 `json:"reach2"`
	BootupTime float64 `json:"bootup"`
	PeerTTL    float64 `json:"ttl"`
	DeathRate  float64 `json:"deathRate"`
}

// RenderCfg options
type RenderCfg struct {
	Mode    string `json:"mode"`
	Source  string `json:"source"`
	File    string `json:"file"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Dynamic bool   `json:"dynamic"`
}

// Option for comtrol flags/values
type Option struct {
	MaxRepeat  int  `json:"maxRepeat"`
	StopOnLoop bool `json:"stopOnLoop"`
}

// Config for test configuration data
type Config struct {
	Core    *core.Config `json:"core"`
	Env     *EnvironCfg  `json:"environment"`
	Node    *NodeCfg     `json:"node"`
	Options *Option      `json:"options"`
	Render  *RenderCfg   `json:"render"`
}

// Cfg is the global configuration
var Cfg = &Config{
	Core: &core.Config{
		MaxTeachs: 10,
		TTLEntry:  60,
		LearnIntv: 10,
	},
	Env: &EnvironCfg{
		Width:    100.,
		Height:   100.,
		NumNodes: 60,
		CoolDown: 5,
	},
	Node: &NodeCfg{
		Reach2:     500.,
		BootupTime: 60.,
		PeerTTL:    600.,
		DeathRate:  0.,
	},
	Options: &Option{
		MaxRepeat: 0,
	},
	Render: &RenderCfg{
		Mode:   "none",
		File:   "",
		Source: "rtab",
	},
}

//----------------------------------------------------------------------

// ReadConfig to deserialize a configuration from a JSON file
func ReadConfig(fn string) error {
	data, err := os.ReadFile(fn)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &Cfg)
}
