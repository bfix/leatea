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
	_ "embed"
	"fmt"
	"image/color"
	"math"
	"os"

	svg "github.com/ajstarks/svgo"
	"github.com/tfriedel6/canvas"
	"github.com/tfriedel6/canvas/sdlcanvas"
)

// Color definitions for drawing
var (
	ClrWhite = &color.RGBA{255, 255, 255, 0}
	ClrRed   = &color.RGBA{255, 0, 0, 0}
	ClrRedTr = &color.RGBA{255, 0, 0, 224}
	ClrBlack = &color.RGBA{0, 0, 0, 0}
	ClrGray  = &color.RGBA{240, 240, 240, 0}
	ClrBlue  = &color.RGBA{0, 0, 255, 0}
)

// Canvas for drawing the network diagram and environment
type Canvas interface {
	// Open a canvas (prepare resources)
	Open() error

	// Start a new rendering
	Render(func(Canvas))

	// Circle primitive
	Circle(x, y, r, w float64, clrBorder, clrFill *color.RGBA)

	// Text primitive
	Text(x, y, fs float64, s, anchor string)

	// Line primitive
	Line(x1, y1, x2, y2, w float64, clr *color.RGBA)

	// IsDynamic returns true if the canvas can draw a
	// sequence of renderings (like UI or video canvases)
	IsDynamic() bool

	// Close a canvas. No further operations are allowed
	Close() error
}

// GetCanvas returns a canvas for drawing (factory)
func GetCanvas(cfg *RenderCfg) (c Canvas) {
	switch cfg.Mode {
	case "svg":
		c = NewSVGCanvas(Cfg.Render.File, Cfg.Env.Width, Cfg.Env.Height, math.Sqrt(Cfg.Node.Reach2))
	case "sdl":
		c = NewSDLCanvas(Cfg.Env.Width, Cfg.Env.Height, math.Sqrt(Cfg.Node.Reach2))
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
func (c *SVGCanvas) Open() error {
	c.svg = svg.New(c.buf)
	return nil
}

// IsDynamic returns true if the canvas can draw a
// sequence of renderings (like UI or video canvases)
func (c *SVGCanvas) IsDynamic() bool {
	return false
}

// Start the canvas (new rendering begins)
func (c *SVGCanvas) Render(proc func(Canvas)) {
	c.svg.Start(c.w, c.h)
	proc(c)
	c.svg.End()
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

// Close a canvas. No further operations are allowed
func (c *SVGCanvas) Close() (err error) {
	// write to file
	if len(c.fn) > 0 {
		var f *os.File
		if f, err = os.Create(c.fn); err == nil {
			_, err = f.Write(c.buf.Bytes())
			f.Close()
		}
	}
	c.buf = nil
	return
}

//----------------------------------------------------------------------
// SDL canvas
//----------------------------------------------------------------------

//go:embed ankacoder.ttf
var font []byte

// SDLCanvas for windowed display
type SDLCanvas struct {
	w, h, off  float64
	scale      float64
	offX, offY float64
	win        *sdlcanvas.Window
	cv         *canvas.Canvas
}

// NewSDLCanvas creates a new SDL canvas for display
func NewSDLCanvas(w, h, off float64) *SDLCanvas {
	c := new(SDLCanvas)
	c.w = w
	c.h = h
	c.off = off
	return c
}

// Open a canvas (prepare resources)
func (c *SDLCanvas) Open() (err error) {
	// create window
	c.win, c.cv, err = sdlcanvas.CreateWindow(Cfg.Render.Width, Cfg.Render.Height, "LEArn/TEAch routing")
	// load font
	_, _ = c.cv.LoadFont(font)
	return
}

// IsDynamic returns true if the canvas can draw a
// sequence of renderings (like UI or video canvases)
func (c *SDLCanvas) IsDynamic() bool {
	return Cfg.Render.Dynamic
}

// Start the canvas (new rendering begins)
func (c *SDLCanvas) Render(proc func(Canvas)) {
	c.win.MainLoop(func() {
		// compute best scale
		w, h := float64(c.cv.Width()), float64(c.cv.Height())
		sw := w / (c.w + 2*c.off)
		sh := h / (c.h + 2*c.off)
		if sw > sh {
			c.scale = sh
			c.offX = (w - c.w*sh) / 2
			c.offY = c.off * sh
		} else {
			c.scale = sw
			c.offX = c.off * sw
			c.offY = (h - c.h*sh) / 2
		}
		// clear screen
		c.cv.SetFillStyle("#FFF")
		c.cv.FillRect(0, 0, w, h)
		proc(c)
	})
}

// Circle primitive
func (c *SDLCanvas) Circle(x, y, r, w float64, clrBorder, clrFill *color.RGBA) {
	cx, cy := c.xlate(x, y)
	cr := c.scale * r
	cw := c.scale * w
	if clrFill != nil {
		c.cv.SetFillStyle(clrFill.R, clrFill.G, clrFill.B)
		c.cv.BeginPath()
		c.cv.Arc(cx, cy, cr, 0, math.Pi*2, false)
		c.cv.ClosePath()
		c.cv.Fill()
	}
	if clrBorder != nil {
		c.cv.SetStrokeStyle(clrBorder.R, clrBorder.G, clrBorder.B)
		c.cv.SetLineWidth(cw)
		c.cv.BeginPath()
		c.cv.Arc(cx, cy, cr, 0, math.Pi*2, false)
		c.cv.ClosePath()
		c.cv.Stroke()
	}
}

// Text primitive
func (c *SDLCanvas) Text(x, y, fs float64, s, anchor string) {
	cx, cy := c.xlate(x, y)
	cfs := c.scale * fs
	c.cv.SetFillStyle(0, 0, 0)
	c.cv.SetTextAlign(canvas.Center)
	c.cv.SetTextBaseline(canvas.Middle)
	c.cv.SetFont(nil, cfs)
	c.cv.FillText(s, cx, cy)
}

// Line primitive
func (c *SDLCanvas) Line(x1, y1, x2, y2, w float64, clr *color.RGBA) {
	cx1, cy1 := c.xlate(x1, y1)
	cx2, cy2 := c.xlate(x2, y2)
	cw := c.scale * w
	c.cv.SetStrokeStyle(clr.R, clr.G, clr.B)
	c.cv.SetLineWidth(cw)
	c.cv.BeginPath()
	c.cv.MoveTo(cx1, cy1)
	c.cv.LineTo(cx2, cy2)
	c.cv.ClosePath()
	c.cv.Stroke()
}

// coordinate translation
func (c *SDLCanvas) xlate(x, y float64) (float64, float64) {
	return x*c.scale + c.offX, y*c.scale + c.offY
}

// Close a canvas. No further operations are allowed
func (c *SDLCanvas) Close() error {
	return nil
}
