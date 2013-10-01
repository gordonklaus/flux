package main

import (
	"code.google.com/p/go.exp/go/types"
	. "code.google.com/p/gordon-go/gui"
)

type ifNode struct {
	*ViewBase
	AggregateMouser
	blk           *block
	input         *port
	seqIn, seqOut *port
	falseblk      *block
	trueblk       *block
	focused       bool
}

func newIfNode() *ifNode {
	n := &ifNode{}
	n.ViewBase = NewView(n)
	n.AggregateMouser = AggregateMouser{NewClickFocuser(n), NewMover(n)}
	n.input = newInput(n, &types.Var{Type: types.Typ[types.Bool]})
	n.falseblk = newBlock(n)
	n.trueblk = newBlock(n)
	n.Add(n.input)
	n.Add(n.falseblk)
	n.Add(n.trueblk)

	n.seqIn = newInput(n, &types.Var{Name: "seq", Type: seqType})
	MoveCenter(n.seqIn, Pt(-portSize, 0))
	n.Add(n.seqIn)
	n.seqOut = newOutput(n, &types.Var{Name: "seq", Type: seqType})
	MoveCenter(n.seqOut, Pt(portSize, 0))
	n.Add(n.seqOut)

	MoveCenter(n.input, Pt(-2*portSize, 0))
	n.update()
	return n
}

func (n ifNode) block() *block      { return n.blk }
func (n *ifNode) setBlock(b *block) { n.blk = b }
func (n ifNode) inputs() []*port    { return []*port{n.seqIn, n.input} }
func (n ifNode) outputs() []*port   { return []*port{n.seqOut} }

func (n ifNode) inConns() []*connection {
	return append(append(append(n.input.conns, n.seqIn.conns...), n.falseblk.inConns()...), n.trueblk.inConns()...)
}

func (n ifNode) outConns() []*connection {
	return append(append(n.seqOut.conns, n.falseblk.outConns()...), n.trueblk.outConns()...)
}

func (n *ifNode) update() bool {
	f, t := !n.falseblk.update(), !n.trueblk.update()
	if f && t {
		return false
	}
	n.falseblk.Move(Pt(-blockRadius, -4-Height(n.falseblk)))
	n.trueblk.Move(Pt(-blockRadius, 4))
	ResizeToFit(n, 0)
	return true
}

func (n *ifNode) Move(p Point) {
	n.ViewBase.Move(p)
	nodeMoved(n)
}

func (n *ifNode) TookKeyFocus() { n.focused = true; Repaint(n) }
func (n *ifNode) LostKeyFocus() { n.focused = false; Repaint(n) }

func (n *ifNode) KeyPress(event KeyEvent) {
	switch event.Key {
	case KeyEscape:
		SetKeyFocus(n.blk)
	default:
		n.ViewBase.KeyPress(event)
	}
}

func (n ifNode) Paint() {
	SetColor(map[bool]Color{false: {.5, .5, .5, 1}, true: {.3, .3, .7, 1}}[n.focused])
	DrawLine(CenterInParent(n.input), CenterInParent(n.seqOut))
	DrawLine(Pt(0, -4), Pt(0, 4))
}
