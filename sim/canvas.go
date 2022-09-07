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
	"fmt"
	"image/color"
	"io"

	svg "github.com/ajstarks/svgo"
)

// Color definitions for drawing
var (
	ClrRed   = &color.RGBA{255, 0, 0, 0}
	ClrBlack = &color.RGBA{0, 0, 0, 0}
	ClrBlue  = &color.RGBA{0, 0, 255, 0}
)

// Canvas for drawing the network diagram and environment
type Canvas interface {
	Start(w, h, margin, prec float64)
	Circle(x, y, r, w float64, clrBorder, clrFill *color.RGBA)
	Text(x, y, fs float64, s, anchor string)
	Line(x1, y1, x2, y2, w float64, clr *color.RGBA)
	End()
}

//----------------------------------------------------------------------
// SVG canvas
//----------------------------------------------------------------------

// SVGCanvas for writing SVG streams
type SVGCanvas struct {
	off, prec float64
	svg       *svg.SVG
}

// Start the canvas
func (c *SVGCanvas) Start(w, h, margin, prec float64) {
	c.off = margin
	c.prec = prec
	c.svg.Start(c.xlate(w+margin), c.xlate(h+margin))
}

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

func (c *SVGCanvas) Text(x, y, fs float64, s, anchor string) {
	style := fmt.Sprintf("text-anchor:%s;font-size:%dpx", anchor, int(fs/c.prec))
	c.svg.Text(c.xlate(x), c.xlate(y), s, style)
}

func (c *SVGCanvas) Line(x1, y1, x2, y2, w float64, clr *color.RGBA) {
	style := "stroke:black;stroke-width:1"
	if w > 0 && clr != nil {
		style = fmt.Sprintf("stroke:#%02x%02x%02x;stroke-width:%d;",
			clr.R, clr.G, clr.B, int(w/c.prec))
	}
	c.svg.Line(c.xlate(x1), c.xlate(y1), c.xlate(x2), c.xlate(y2), style)
}

func (c *SVGCanvas) xlate(x float64) int {
	return int((x + c.off) / c.prec)
}

func (c *SVGCanvas) End() {
	c.svg.End()
}

func NewSVGCanvas(wrt io.Writer, w, h, off float64) *SVGCanvas {
	c := new(SVGCanvas)
	c.svg = svg.New(wrt)
	c.Start(w, h, off, 0.01)
	return c
}
