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
	"bytes"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"

	svg "github.com/ajstarks/svgo"
)

// Color definitions for drawing
var (
	ClrWhite = &color.RGBA{255, 255, 255, 0}
	ClrRed   = &color.RGBA{255, 0, 0, 0}
	ClrRedTr = &color.RGBA{255, 0, 0, 224}
	ClrBlack = &color.RGBA{0, 0, 0, 0}
	ClrBlue  = &color.RGBA{0, 0, 255, 0}
)

// Canvas for drawing the network diagram and environment
type Canvas interface {
	// Open a canvas (prepare resources)
	Open()

	// Start a new graph
	Start()

	// Circle primitive
	Circle(x, y, r, w float64, clrBorder, clrFill *color.RGBA)

	// Text primitive
	Text(x, y, fs float64, s, anchor string)

	// Line primitive
	Line(x1, y1, x2, y2, w float64, clr *color.RGBA)

	// Finalise graph
	End()

	// IsDynamic returns true if the canvas can draw a
	// sequence of renderings (like UI or video canvases)
	IsDynamic() bool

	// Close a canvas. No further operations are allowed
	Close()
}

// GetCanvas returns a canvas for drawing (factory)
func GetCanvas(cfg *RenderCfg) (c Canvas) {
	switch cfg.Mode {
	case "svg":
		c = NewSVGCanvas(Cfg.Render.File, Cfg.Env.Width, Cfg.Env.Height, math.Sqrt(Cfg.Node.Reach2))
	}
	return
}

//----------------------------------------------------------------------
// SVG canvas
//----------------------------------------------------------------------

// SVGCanvas for writing SVG streams
type SVGCanvas struct {
	off, prec float64
	svg       *svg.SVG
	w, h      int
	buf       *bytes.Buffer
	fn        string
}

// NewSVGCanvas creates a new SVG canvas to be stored in a file
func NewSVGCanvas(fn string, w, h, off float64) *SVGCanvas {
	c := new(SVGCanvas)
	c.buf = new(bytes.Buffer)
	c.fn = fn
	c.off = off
	c.prec = 0.01
	c.w = c.xlate(w + off)
	c.h = c.xlate(h + off)
	return c
}

// Open a canvas (prepare resources)
func (c *SVGCanvas) Open() {
	c.svg = svg.New(c.buf)
}

// IsDynamic returns true if the canvas can draw a
// sequence of renderings (like UI or video canvases)
func (c *SVGCanvas) IsDynamic() bool {
	return false
}

// Start the canvas (new rendering begins)
func (c *SVGCanvas) Start() {
	c.svg.Start(c.w, c.h)
}

// Circle primitive
func (c *SVGCanvas) Circle(x, y, r, w float64, clrBorder, clrFill *color.RGBA) {
	fill := "none"
	if clrFill != nil {
		fill = fmt.Sprintf("#%02x%02x%02x", clrFill.R, clrFill.G, clrFill.B)
	}
	border := ""
	if w > 0 && clrBorder != nil {
		border = fmt.Sprintf("stroke:#%02x%02x%02x;stroke-width:%d;",
			clrBorder.R, clrBorder.G, clrBorder.B, int(w/c.prec))
	}
	style := fmt.Sprintf("%sfill:%s", border, fill)
	c.svg.Circle(c.xlate(x), c.xlate(y), int(r/c.prec), style)
}

// Text primitive
func (c *SVGCanvas) Text(x, y, fs float64, s, anchor string) {
	style := fmt.Sprintf("text-anchor:%s;font-size:%dpx", anchor, int(fs/c.prec))
	c.svg.Text(c.xlate(x), c.xlate(y), s, style)
}

// Line primitive
func (c *SVGCanvas) Line(x1, y1, x2, y2, w float64, clr *color.RGBA) {
	style := "stroke:black;stroke-width:1"
	if w > 0 && clr != nil {
		style = fmt.Sprintf("stroke:#%02x%02x%02x;stroke-width:%d;",
			clr.R, clr.G, clr.B, int(w/c.prec))
	}
	c.svg.Line(c.xlate(x1), c.xlate(y1), c.xlate(x2), c.xlate(y2), style)
}

// coordinate translation
func (c *SVGCanvas) xlate(x float64) int {
	return int((x + c.off) / c.prec)
}

// Finalize graph
func (c *SVGCanvas) End() {
	c.svg.End()
	// write to file
	if len(c.fn) > 0 {
		f, err := os.Create(c.fn)
		if err != nil {
			log.Printf("file: %s", c.fn)
			log.Fatal(err)
		}
		defer f.Close()
		_, _ = f.Write(c.buf.Bytes())
	}
}

// Close a canvas. No further operations are allowed
func (c *SVGCanvas) Close() {
	c.buf = nil
}
