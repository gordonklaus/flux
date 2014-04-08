// Copyright 2014 Gordon Klaus. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/gordon-go/go/types"
	. "code.google.com/p/gordon-go/gui"
)

type appendNode struct {
	*nodeBase
}

func newAppendNode() *appendNode {
	n := &appendNode{}
	n.nodeBase = newNodeBase(n)
	n.text.SetText("append")
	n.text.SetTextColor(color(&types.Func{}, true, false))
	n.addSeqPorts()
	in := n.newInput(newVar("", nil))
	out := n.newOutput(newVar("", nil))
	in.connsChanged = func() {
		t := inputType(in)
		in.setType(t)
		if t == nil {
			for _, p := range ins(n)[1:] {
				n.removePortBase(p)
			}
		} else if n.ellipsis() {
			p := ins(n)[1]
			p.valView.ellipsis = true
			p.setType(t)
		} else {
			for _, p := range ins(n)[1:] {
				p.setType(underlying(t).(*types.Slice).Elem)
			}
		}
		out.setType(t)
	}
	return n
}

func (n *appendNode) connectable(t types.Type, dst *port) bool {
	if dst == ins(n)[0] {
		_, ok := underlying(t).(*types.Slice)
		return ok
	}
	return assignable(t, dst.obj.Type)
}

func (n *appendNode) KeyPress(event KeyEvent) {
	ins := ins(n)
	v := ins[0].obj
	t, ok := v.Type.(*types.Slice)
	if ok && event.Key == KeyComma {
		if n.ellipsis() {
			n.removePortBase(ins[1])
		}
		SetKeyFocus(n.newInput(newVar("", t.Elem)))
	} else if ok && event.Key == KeyPeriod && event.Ctrl {
		if n.ellipsis() {
			n.removePortBase(ins[1])
		} else {
			for _, p := range ins[1:] {
				n.removePortBase(p)
			}
			p := n.newInput(v)
			p.valView.ellipsis = true
			p.valView.refresh()
			SetKeyFocus(p)
		}
	} else {
		n.ViewBase.KeyPress(event)
	}
}

func (n *appendNode) removePort(p *port) {
	for _, p2 := range ins(n)[1:] {
		if p2 == p {
			n.removePortBase(p)
			break
		}
	}
}

func (n *appendNode) ellipsis() bool {
	ins := ins(n)
	return len(ins) == 2 && ins[1].obj == ins[0].obj
}

type deleteNode struct {
	*nodeBase
}

func newDeleteNode() *deleteNode {
	n := &deleteNode{}
	n.nodeBase = newNodeBase(n)
	n.text.SetText("delete")
	n.text.SetTextColor(color(&types.Func{}, true, false))
	m := n.newInput(newVar("map", nil))
	key := n.newInput(newVar("key", nil))
	m.connsChanged = func() {
		t := inputType(m)
		m.setType(t)
		if t != nil {
			key.setType(underlying(t).(*types.Map).Key)
		} else {
			key.setType(nil)
		}
	}
	n.addSeqPorts()
	return n
}

func (n *deleteNode) connectable(t types.Type, dst *port) bool {
	if dst == ins(n)[0] {
		_, ok := underlying(t).(*types.Map)
		return ok
	}
	return assignable(t, dst.obj.Type)
}

type lenNode struct {
	*nodeBase
}

func newLenNode() *lenNode {
	n := &lenNode{}
	n.nodeBase = newNodeBase(n)
	n.text.SetText("len")
	n.text.SetTextColor(color(&types.Func{}, true, false))
	in := n.newInput(newVar("", nil))
	n.newOutput(newVar("", types.Typ[types.Int]))
	in.connsChanged = func() {
		in.setType(inputType(in))
	}
	n.addSeqPorts()
	return n
}

func (n *lenNode) connectable(t types.Type, dst *port) bool {
	ok := false
	switch t := underlying(t).(type) {
	case *types.Array, *types.Slice:
		ok = true
	case *types.Pointer:
		_, ok = underlying(t.Elem).(*types.Array)
	}
	return ok
}

type makeNode struct {
	*nodeBase
	typ *typeView
}

func newMakeNode() *makeNode {
	n := &makeNode{}
	n.nodeBase = newNodeBase(n)
	out := n.newOutput(nil)
	n.typ = newTypeView(&out.obj.Type)
	n.typ.mode = makeableType
	n.Add(n.typ)
	return n
}

func (n *makeNode) editType() {
	n.typ.editType(func() {
		if t := *n.typ.typ; t != nil {
			n.setType(t)
		} else {
			n.blk.removeNode(n)
			SetKeyFocus(n.blk)
		}
	})
}

func (n *makeNode) setType(t types.Type) {
	n.typ.setType(t)
	n.outs[0].setType(t)
	if t != nil {
		n.blk.func_().addPkgRef(t)
		if nt, ok := t.(*types.Named); ok {
			t = nt.UnderlyingT
		}
		n.newInput(newVar("len", types.Typ[types.Int]))
		if _, ok := t.(*types.Slice); ok {
			n.newInput(newVar("cap", types.Typ[types.Int]))
		}
		MoveCenter(n.typ, ZP)
		n.gap = Height(n.typ) / 2
		n.reform()
		SetKeyFocus(n)
	}
}

func newVar(name string, typ types.Type) *types.Var {
	return types.NewVar(0, nil, name, typ)
}