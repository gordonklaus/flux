package main

import (
	. "code.google.com/p/gordon-go/gui"
	"math"
)

type connection struct {
	*ViewBase
	AggregateMouser
	blk      *block
	src      *port
	dst      *port
	feedback bool

	srcHandle *connectionSourceHandle
	dstHandle *connectionDestinationHandle

	focused bool
	srcPt   Point
	dstPt   Point
}

func newConnection() *connection {
	c := &connection{}
	c.ViewBase = NewView(c)
	c.AggregateMouser = AggregateMouser{NewClickFocuser(c)}
	c.srcHandle = newConnectionSourceHandle(c)
	c.dstHandle = newConnectionDestinationHandle(c)
	c.Add(c.srcHandle)
	c.Add(c.dstHandle)
	return c
}

func (c connection) connected() bool { return c.src != nil && c.dst != nil }
func (c *connection) disconnect() {
	if c.src != nil {
		c.src.disconnect(c)
		c.src.connsChanged()
		c.src = nil
	}
	if c.dst != nil {
		c.dst.disconnect(c)
		c.dst.connsChanged()
		c.dst = nil
	}
}

func (c *connection) setSrc(src *port) {
	if c.src != nil {
		c.src.disconnect(c)
		c.src.connsChanged()
	}
	c.src = src
	if src != nil {
		src.connect(c)
		if c.dst != nil {
			c.src.connsChanged()
			c.dst.connsChanged()
		}
	}
	c.reblock()
	c.reform()
}

func (c *connection) setDst(dst *port) {
	if c.dst != nil {
		c.dst.disconnect(c)
		c.dst.connsChanged()
	}
	c.dst = dst
	if dst != nil {
		dst.connect(c)
		if c.src != nil {
			c.src.connsChanged()
			c.dst.connsChanged()
		}
	}
	c.reblock()
	c.reform()
}

func (c *connection) reblock() {
	var newblk *block
	if c.src == nil && c.dst == nil {
		return
	} else if c.src == nil {
		newblk = c.dst.node.block()
	} else if c.dst == nil {
		newblk = c.src.node.block()
	} else {
	loop:
		for srcblk := c.src.node.block(); srcblk != nil; srcblk = srcblk.outer() {
			for dstblk := c.dst.node.block(); dstblk != nil; dstblk = dstblk.outer() {
				if srcblk == dstblk {
					newblk = srcblk
					break loop
				}
			}
		}
	}
	newblk.addConn(c)
}

func (c *connection) reform() {
	unconnectedOffset := Pt(48, 0)
	if c.feedback {
		unconnectedOffset.X = -208
	}
	if c.src != nil {
		c.srcPt = MapTo(c.src, Center(c.src), c.blk)
	} else {
		c.srcPt = c.dstPt.Sub(unconnectedOffset)
	}
	if c.dst != nil {
		c.dstPt = MapTo(c.dst, Center(c.dst), c.blk)
	} else {
		c.dstPt = c.srcPt.Add(unconnectedOffset)
	}

	var rect Rectangle
	if c.src != nil && c.src.obj.Type == seqType || c.dst != nil && c.dst.obj.Type == seqType {
		rect = Rectangle{Pt(c.srcPt.X, math.Min(c.srcPt.Y, c.dstPt.Y)-25), Pt(c.dstPt.X, math.Max(c.srcPt.Y, c.dstPt.Y))}
	} else {
		rect = Rectangle{c.srcPt, c.dstPt}.Canon()
	}
	c.Move(rect.Min)
	c.SetRect(rect)

	handleOffset := c.dstPt.Sub(c.srcPt).Div(4)
	if c.srcHandle.editing {
		MoveCenter(c.srcHandle, MapFromParent(c, c.srcPt))
	} else {
		MoveCenter(c.srcHandle, MapFromParent(c, c.srcPt.Add(handleOffset)))
	}
	if c.dstHandle.editing {
		MoveCenter(c.dstHandle, MapFromParent(c, c.dstPt))
	} else {
		MoveCenter(c.dstHandle, MapFromParent(c, c.dstPt.Sub(handleOffset)))
	}
}

func (c *connection) startEditing() {
	if c.src == nil {
		c.srcHandle.startEditing()
	} else {
		c.dstHandle.startEditing()
	}
}

func (c *connection) TookKeyFocus() { c.focused = true; Repaint(c) }
func (c *connection) LostKeyFocus() { c.focused = false; Repaint(c) }

func (c *connection) KeyPress(event KeyEvent) {
	switch event.Key {
	case KeyLeft:
		SetKeyFocus(c.src)
	case KeyRight:
		SetKeyFocus(c.dst)
	case KeyDown, KeyUp:
		c.blk.outermost().focusNearestView(c, event.Key)
	case KeyBackspace:
		SetKeyFocus(c.src)
		c.blk.removeConn(c)
	case KeyDelete:
		SetKeyFocus(c.dst)
		c.blk.removeConn(c)
	case KeyEscape:
		SetKeyFocus(c.blk)
	default:
		if event.Text == "\\" {
			c.feedback = !c.feedback
			c.reform()
		} else {
			c.ViewBase.KeyPress(event)
		}
	}
}

func (c *connection) Paint() {
	SetColor(map[bool]Color{false: {.5, .5, .5, 1}, true: {.3, .3, .7, 1}}[c.focused])
	start, end := MapFromParent(c, c.srcPt), MapFromParent(c, c.dstPt)
	var pts []Point
	if c.src != nil && c.src.obj.Type == seqType || c.dst != nil && c.dst.obj.Type == seqType {
		pts = []Point{start, Pt(start.X, start.Y-40), Pt(end.X, end.Y-40), end}
	} else {
		d := end.Sub(start)
		mid := start.Add(d.Div(2))
		if c.feedback {
			mid.Y = math.Max(start.Y, end.Y) + 128
		}
		dx := math.Abs(d.X / 3)
		p1 := start.Add(Pt(dx, 0))
		p2 := mid
		p3 := end.Sub(Pt(dx, 0))
		pts = []Point{start, p1, p2, p3, end}
	}
	len := 0.0
	for i := range pts {
		if i > 0 {
			len += pts[i].Sub(pts[i-1]).Len()
		}
	}
	DrawBezier(pts, int(len))
}
