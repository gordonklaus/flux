// Copyright 2014 Gordon Klaus. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/gordon-go/go/types"
	. "code.google.com/p/gordon-go/gui"
	"fmt"
	"go/ast"
	"go/build"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type browser struct {
	*ViewBase
	mode       browserMode
	fun        *funcNode
	currentPkg *types.Package
	imports    []*types.Package
	finished   bool
	accepted   func(types.Object)
	canceled   func()

	path    []types.Object
	indices []int
	i       int
	newObj  types.Object

	pathTexts, nameTexts []Text
	text                 *nodeNameText
	typeView             *typeView
	pkgNameText          *TextBase
	funcAsVal            bool
}

type browserMode int

const (
	browse browserMode = iota
	fluxSourceOnly
	typesOnly
	compositeOrPtrType
	compositeType
	makeableType
)

func newBrowser(mode browserMode, context View) *browser {
	b := &browser{mode: mode, accepted: func(types.Object) {}, canceled: func() {}}
	b.ViewBase = NewView(b)

	switch v := context.(type) {
	case *funcNode:
		b.fun = v
		b.currentPkg = v.pkg()
		b.imports = v.imports()
	case *typeView:
		// TODO: get pkg and imports for the root of this typeView
		for v := View(v); v != nil; v = Parent(v) {
			if blk, ok := v.(*block); ok {
				f := blk.func_()
				b.currentPkg = f.pkg()
				b.imports = f.imports()
				break
			}
		}
	}

	b.text = newNodeNameText(b)
	b.text.SetBackgroundColor(Color{0, 0, 0, 0})
	b.Add(b.text)

	b.pkgNameText = NewText("")
	b.pkgNameText.SetBackgroundColor(Color{0, 0, 0, .7})
	b.Add(b.pkgNameText)

	b.text.SetText("")

	return b
}

func (b *browser) cancel() {
	if !b.finished {
		b.finished = true
		b.canceled()
	}
}

type special struct {
	types.Object
	name string
}

func (s special) GetName() string { return s.name }

type buildPackage struct {
	types.Object
	*build.Package
}

func (p buildPackage) GetName() string {
	if p.Dir == "" {
		return ""
	}
	return path.Base(p.Dir)
}

type field struct {
	*types.Var
	recv types.Type
}

type objects []types.Object

func (o objects) Len() int { return len(o) }
func (o objects) Less(i, j int) bool {
	return objLess(o[i], o[j])
}
func objLess(o1, o2 types.Object) bool {
	n1, n2 := o1.GetName(), o2.GetName()
	switch o1.(type) {
	case special:
		switch o2.(type) {
		case special:
			return n1 < n2
		default:
			return true
		}
	case *types.TypeName:
		switch o2.(type) {
		case special:
			return false
		case *types.TypeName:
			return n1 < n2
		default:
			return true
		}
	case *types.Func, *types.Builtin:
		switch o2.(type) {
		case special, *types.TypeName:
			return false
		case *types.Func, *types.Builtin:
			return n1 < n2
		default:
			return true
		}
	case *types.Var, field, *localVar:
		switch o2.(type) {
		default:
			return false
		case *types.Var, field, *localVar:
			return n1 < n2
		case *types.Const, *types.Nil, buildPackage:
			return true
		}
	case *types.Const, *types.Nil:
		switch o2.(type) {
		default:
			return false
		case *types.Const, *types.Nil:
			return n1 < n2
		case buildPackage:
			return true
		}
	case buildPackage:
		switch o2.(type) {
		default:
			return false
		case buildPackage:
			return n1 < n2
		}
	}
	panic("unreachable")
}
func (o objects) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

var buildPackages = map[string]buildPackage{}

func (b browser) filteredObjs() (objs []types.Object) {
	add := func(obj types.Object) {
		if _, ok := obj.(buildPackage); !ok {
			switch b.mode {
			case fluxSourceOnly:
				if !fluxObjs[obj] {
					return
				}
			case typesOnly:
				if _, ok := obj.(*types.TypeName); !ok {
					return
				}
			case compositeOrPtrType, compositeType:
				switch obj {
				default:
					obj, ok := obj.(*types.TypeName)
					if !ok {
						return
					}
					t, ok := obj.GetType().(*types.Named)
					if !ok {
						return
					}
					switch t.UnderlyingT.(type) {
					default:
						return
					case *types.Array, *types.Slice, *types.Map, *types.Struct:
					}
				case protoPointer:
					if b.mode == compositeType {
						return
					}
				case protoArray, protoSlice, protoMap, protoStruct:
				}
			case makeableType:
				switch obj {
				default:
					obj, ok := obj.(*types.TypeName)
					if !ok {
						return
					}
					t, ok := obj.GetType().(*types.Named)
					if !ok {
						return
					}
					switch t.UnderlyingT.(type) {
					default:
						return
					case *types.Slice, *types.Map, *types.Chan:
					}
				case protoSlice, protoMap, protoChan:
				}
			}
			if b.currentPkg != nil {
				if p := obj.GetPkg(); p != nil && p != b.currentPkg && !ast.IsExported(obj.GetName()) {
					return
				}
			}
		}
		objs = append(objs, obj)
	}

	addPkgs := func(dir string) {
		if file, err := os.Open(dir); err == nil {
			defer file.Close()
			if fileInfos, err := file.Readdir(-1); err == nil {
				for _, fileInfo := range fileInfos {
					name := fileInfo.Name()
					if fileInfo.IsDir() && unicode.IsLetter([]rune(name)[0]) && name != "testdata" {
						fullPath := filepath.Join(dir, name)
						buildPkg, ok := buildPackages[fullPath]
						if !ok {
							pkg, _ := build.ImportDir(fullPath, build.AllowBinary)
							buildPkg = buildPackage{nil, pkg}
							buildPackages[fullPath] = buildPkg
						}
						add(buildPkg)
					}
				}
			}
		}
	}

	if len(b.path) == 0 {
		if b.mode == browse {
			for _, name := range []string{"=", "[]", "[]=", "break", "call", "continue", "convert", "func", "if", "indirect", "loop", "typeAssert"} {
				objs = append(objs, special{name: name})
			}
		}
		pkgs := b.imports
		if b.currentPkg != nil {
			pkgs = append(pkgs, b.currentPkg)
		}
		for _, p := range pkgs {
			for _, obj := range p.Scope().Objects {
				add(obj)
			}
		}
		for _, obj := range types.Universe.Objects {
			add(obj)
		}
		for _, op := range []string{"+", "-", "*", "/", "%", "&", "|", "^", "&^", "&&", "||", "!", "==", "!=", "<", "<=", ">", ">="} {
			add(types.NewFunc(0, nil, op, nil))
		}
		for _, t := range []*types.TypeName{protoPointer, protoArray, protoSlice, protoMap, protoChan, protoFunc, protoInterface, protoStruct} {
			add(t)
		}
		for _, dir := range build.Default.SrcDirs() {
			addPkgs(dir)
		}
		if b.fun != nil {
			b.fun.funcblk.walk(func(blk *block) {
				for v := range blk.localVars {
					add(v)
				}
			}, nil, nil)
		}
	} else {
		switch obj := b.path[0].(type) {
		case buildPackage:
			if pkg, err := getPackage(obj.ImportPath); err == nil {
				for _, obj := range pkg.Scope().Objects {
					add(obj)
				}
			} else {
				if _, ok := err.(*build.NoGoError); !ok {
					fmt.Println(err)
				}
				pkgs[obj.ImportPath] = types.NewPackage(obj.ImportPath, obj.GetName(), &types.Scope{})
			}
			addPkgs(obj.Dir)
		case *types.TypeName:
			// TODO: use types.NewMethodSet to get the entire method set
			// TODO: shouldn't go/types also have a NewFieldSet?
			if nt, ok := obj.Type.(*types.Named); ok {
				for _, m := range nt.Methods {
					add(m)
				}
				switch ut := nt.UnderlyingT.(type) {
				case *types.Struct:
					for _, f := range ut.Fields {
						add(field{f, nt})
					}
				case *types.Interface:
					for _, m := range ut.Methods {
						add(m)
					}
				}
			}
		}
	}

	// TODO: merge duplicate directories across srcDirs (warn if there is package shadowing)

	sort.Sort(objects(objs))
	return
}

func (b browser) currentObj() types.Object {
	objs := b.filteredObjs()
	if len(b.indices) == 0 || len(objs) == 0 {
		return nil
	}
	return objs[b.indices[b.i]]
}

func (b browser) lastPathText() (Text, bool) {
	if np := len(b.pathTexts); np > 0 {
		return b.pathTexts[np-1], true
	}
	return nil, false
}

func (b *browser) Paint() {
	rect := ZR
	if b.newObj == nil && len(b.nameTexts) > 0 {
		cur := b.nameTexts[b.i]
		rect = Rectangle{Pt(0, Pos(cur).Y), Pos(cur).Add(Size(cur))}
	} else {
		rect = RectInParent(b.text)
		rect.Min.X = 0
	}
	SetColor(Color{1, 1, 1, .7})
	FillRect(rect)
}

type nodeNameText struct {
	*TextBase
	b *browser
}

func newNodeNameText(b *browser) *nodeNameText {
	t := &nodeNameText{}
	t.TextBase = NewTextBase(t, "")
	t.b = b
	return t
}
func (t *nodeNameText) SetText(text string) {
	b := t.b
	if b.newObj == nil {
		if objs := b.filteredObjs(); len(objs) > 0 {
			for _, obj := range objs {
				if strings.HasPrefix(strings.ToLower(obj.GetName()), strings.ToLower(text)) {
					goto ok
				}
			}
			return
		}
	}
ok:
	currentIndex := 0
	n := len(b.indices)
	if n > 0 {
		b.i %= n
		if b.i < 0 {
			b.i += n
		}
		currentIndex = b.indices[b.i]
	}

	objs := b.filteredObjs()
	if b.newObj != nil {
		switch obj := b.newObj.(type) {
		case buildPackage:
			obj.Dir = text
		case *types.TypeName:
			obj.Name = text
		case *types.Func:
			obj.Name = text
		case *types.Var:
			obj.Name = text
		case *types.Const:
			obj.Name = text
		case *localVar:
			obj.Name = text
		}
		i := 0
		for i < len(objs) && objLess(objs[i], b.newObj) {
			i++
		}
		objs = append(objs[:i], append([]types.Object{b.newObj}, objs[i:]...)...)
		currentIndex = i
	}

	b.indices = nil
	for i, obj := range objs {
		if strings.HasPrefix(strings.ToLower(obj.GetName()), strings.ToLower(text)) {
			b.indices = append(b.indices, i)
		}
	}
	n = len(b.indices)
	for i, index := range b.indices {
		if index >= currentIndex {
			b.i = i
			break
		}
	}
	if b.i >= n {
		b.i = n - 1
	}

	var cur types.Object
	if n > 0 {
		cur = objs[b.indices[b.i]]
	}
	if cur != nil {
		text = cur.GetName()[:len(text)]
	} else {
		text = ""
	}
	t.TextBase.SetText(text)

	if t, ok := b.lastPathText(); ok && cur != nil {
		sep := ""
		if _, ok := cur.(buildPackage); ok {
			sep = "/"
		} else {
			sep = "."
		}
		text := t.GetText()
		t.SetText(text[:len(text)-1] + sep)
	}
	xOffset := 0.0
	if t, ok := b.lastPathText(); ok {
		xOffset = Pos(t).X + Width(t)
	}

	for _, l := range b.nameTexts {
		l.Close()
	}
	b.nameTexts = []Text{}
	width := 0.0
	for i, activeIndex := range b.indices {
		obj := objs[activeIndex]
		l := NewText(obj.GetName())
		l.SetTextColor(color(obj, false, b.funcAsVal))
		l.SetBackgroundColor(Color{0, 0, 0, .7})
		b.Add(l)
		b.nameTexts = append(b.nameTexts, l)
		l.Move(Pt(xOffset, float64(n-i-1)*Height(l)))
		if Width(l) > width {
			width = Width(l)
		}
	}
	Raise(b.text)
	Resize(b, Pt(xOffset+width, float64(len(b.nameTexts))*Height(b.text)))

	yOffset := float64(n-b.i-1) * Height(b.text)
	b.text.Move(Pt(xOffset, yOffset))
	if b.typeView != nil {
		b.typeView.Close()
	}
	if pkg, ok := cur.(buildPackage); ok {
		t := b.pkgNameText
		t.SetText(pkg.Name)
		t.Move(Pt(xOffset+width+16, yOffset-(Height(t)-Height(b.text))/2))
		if pkg.Name != pkg.GetName() {
			Show(t)
		} else {
			Hide(t)
		}
	} else {
		Hide(b.pkgNameText)
	}
	if cur != nil {
		b.text.SetTextColor(color(cur, true, b.funcAsVal))
		switch cur := cur.(type) {
		case *types.TypeName:
			if t, ok := cur.GetType().(*types.Named); ok {
				b.typeView = newTypeView(&t.UnderlyingT)
				b.Add(b.typeView)
			}
		case *types.Func, *types.Var, *types.Const, field:
			t := cur.GetType()
			b.typeView = newTypeView(&t)
			b.Add(b.typeView)
		case *localVar:
			b.typeView = newTypeView(&cur.Type)
			b.Add(b.typeView)
		}
		if b.typeView != nil {
			b.typeView.Move(Pt(xOffset+width+16, yOffset-(Height(b.typeView)-Height(b.text))/2))
		}
	}
	for _, p := range b.pathTexts {
		p.Move(Pt(Pos(p).X, yOffset))
	}

	Pan(b, Pt(0, yOffset))
}
func (t *nodeNameText) LostKeyFocus() { t.b.cancel() } // TODO: implement this differently.  it interferes with var type editing
func (t *nodeNameText) KeyPress(event KeyEvent) {
	b := t.b
	if b.mode == browse && event.Shift != b.funcAsVal {
		b.funcAsVal = event.Shift
		t.SetText(t.GetText())
	}
	switch event.Key {
	case KeyUp:
		if b.newObj == nil {
			b.i--
			t.SetText(t.GetText())
		}
	case KeyDown:
		if b.newObj == nil {
			b.i++
			t.SetText(t.GetText())
		}
	case KeyBackspace:
		if len(t.GetText()) > 0 {
			t.TextBase.KeyPress(event)
			break
		}
		fallthrough
	case KeyLeft:
		if len(b.path) > 0 && b.newObj == nil {
			previous := b.path[0]
			b.path = b.path[1:]
			b.i = 0
			for i, obj := range b.filteredObjs() {
				if obj == previous {
					b.indices = []int{i}
					break
				}
			}

			i := len(b.pathTexts) - 1
			b.pathTexts[i].Close()
			b.pathTexts = b.pathTexts[:i]

			t.SetText("")
		}
	case KeyEnter:
		cur := b.currentObj()
		if cur == nil && b.newObj == nil {
			return
		}
		if pkg, ok := cur.(buildPackage); ok && event.Shift {
			t := b.pkgNameText
			Show(t)
			t.Accept = func(s string) {
				if s != pkg.Name {
					pkg.Name = s
					savePackageName(pkg.Package)
				}
				b.text.SetText("")
				SetKeyFocus(b.text)
			}
			t.Reject = func() {
				b.text.SetText(b.text.GetText())
				SetKeyFocus(b.text)
			}
			SetKeyFocus(t)
			return
		}

		obj := b.newObj
		existing := false
		if obj == nil {
			obj = cur
		} else if cur != nil && obj.GetName() == cur.GetName() {
			obj = cur
			existing = true
		}
		if b.newObj != nil && !existing {
			switch obj := obj.(type) {
			case buildPackage:
				path := ""
				if len(b.path) > 0 {
					path = b.path[0].(buildPackage).Dir
				} else {
					d := build.Default.SrcDirs()
					path = d[len(d)-1]
				}
				if err := os.Mkdir(filepath.Join(path, obj.GetName()), 0777); err != nil {
					panic(err)
				}
			case *types.TypeName, *types.Func, *types.Var, *types.Const:
				if isMethod(obj) {
					recv := b.path[0].(*types.TypeName).Type.(*types.Named)
					recv.Methods = append(recv.Methods, obj.(*types.Func))
					break
				}
				pkg := b.currentPkg
				if len(b.path) > 0 {
					pkg = pkgs[b.path[0].(buildPackage).ImportPath]
				}
				if pkg != nil {
					pkg.Scope().Insert(obj)
				}
			case *localVar:
				v := b.typeView
				b.finished = true // hack to avoid cancelling browser when it loses keyboard focus
				v.edit(func() {
					b.finished = false
					if *v.typ == nil {
						t.SetText("")
						SetKeyFocus(t)
					} else {
						b.finished = true
						b.accepted(obj)
					}
				})
				b.newObj = nil
				return
			}

			b.i = 0
			for i, child := range b.filteredObjs() {
				if child == obj {
					b.indices = []int{i}
					break
				}
			}
		}
		b.newObj = nil

		_, isPkg := obj.(buildPackage)
		_, isType := obj.(*types.TypeName)
		if !(isPkg || b.mode == browse && isType) {
			b.finished = true
			b.accepted(obj)
			return
		}
		fallthrough
	case KeyRight:
		if b.newObj == nil {
			switch obj := b.currentObj().(type) {
			case buildPackage, *types.TypeName:
				if t, ok := obj.(*types.TypeName); ok {
					if _, ok = t.Type.(*types.Basic); ok || t.Type == nil {
						break
					}
				}
				b.path = append([]types.Object{obj}, b.path...)
				b.indices = nil

				sep := ""
				if _, ok := obj.(buildPackage); ok {
					sep = "/"
				} else {
					sep = "."
				}
				pathText := NewText(obj.GetName() + sep)
				pathText.SetTextColor(color(obj, true, b.funcAsVal))
				pathText.SetBackgroundColor(Color{0, 0, 0, .7})
				b.Add(pathText)
				x := 0.0
				if t, ok := b.lastPathText(); ok {
					x = Pos(t).X + Width(t)
				}
				pathText.Move(Pt(x, 0))
				b.pathTexts = append(b.pathTexts, pathText)

				t.SetText("")
			}
		}
	case KeyEscape:
		if b.newObj == nil {
			b.cancel()
		} else {
			b.newObj = nil
			t.SetText("")
		}
	default:
		if b.fun != nil {
			if event.Command && event.Text == "0" {
				b.newObj = &localVar{refs: map[*valueNode]bool{}}
				t.SetText("")
			} else {
				t.TextBase.KeyPress(event)
			}
			return
		}

		makeInPkg := false
		var pkg *types.Package
		var recv *types.TypeName
		if len(b.path) > 0 {
			switch obj := b.path[0].(type) {
			case buildPackage:
				makeInPkg = true
				pkg = pkgs[obj.ImportPath]
			case *types.TypeName:
				recv = obj
				pkg = obj.Pkg
			}
		}
		makePkgInRoot := len(b.path) == 0 && event.Text == "1"
		makeMethod := recv != nil && event.Text == "3"
		if b.newObj == nil && b.mode != typesOnly && event.Command && (makePkgInRoot || makeInPkg || makeMethod) {
			switch event.Text {
			case "1":
				b.newObj = buildPackage{nil, &build.Package{}}
			case "2":
				t := types.NewTypeName(0, pkg, "", nil)
				t.Type = &types.Named{Obj: t}
				b.newObj = t
			case "3":
				sig := &types.Signature{}
				if recv != nil {
					sig.Recv = newVar("", &types.Pointer{Elem: recv.Type})
				}
				b.newObj = types.NewFunc(0, pkg, "", sig)
			case "4":
				b.newObj = types.NewVar(0, pkg, "", nil)
			case "5":
				b.newObj = types.NewConst(0, pkg, "", nil, nil)
			default:
				t.TextBase.KeyPress(event)
				return
			}
			t.SetText("")
		} else {
			t.TextBase.KeyPress(event)
		}
	}
}

func (t *nodeNameText) KeyRelease(event KeyEvent) {
	b := t.b
	if b.mode == browse && event.Shift != b.funcAsVal {
		b.funcAsVal = event.Shift
		t.SetText(t.GetText())
	}
}

func color(obj types.Object, bright, funcAsVal bool) Color {
	alpha := .7
	if bright {
		alpha = 1
	}
	switch obj.(type) {
	case special:
		return Color{1, 1, .6, alpha}
	case buildPackage:
		return Color{1, 1, 1, alpha}
	case *types.TypeName:
		return Color{.6, 1, .6, alpha}
	case *types.Func, *types.Builtin:
		if funcAsVal && obj.GetPkg() != nil { //Pkg==nil == builtin
			return color(&types.Var{}, bright, funcAsVal)
		}
		return Color{1, .6, .6, alpha}
	case *types.Var, *types.Const, *types.Nil, field, *localVar:
		return Color{.6, .6, 1, alpha}
	}
	panic(fmt.Sprintf("unknown object type %T", obj))
}

var (
	protoPointer   = types.NewTypeName(0, nil, "pointer", nil)
	protoArray     = types.NewTypeName(0, nil, "array", nil)
	protoSlice     = types.NewTypeName(0, nil, "slice", nil)
	protoMap       = types.NewTypeName(0, nil, "map", nil)
	protoChan      = types.NewTypeName(0, nil, "chan", nil)
	protoFunc      = types.NewTypeName(0, nil, "func", nil)
	protoInterface = types.NewTypeName(0, nil, "interface", nil)
	protoStruct    = types.NewTypeName(0, nil, "struct", nil)
)

func newProtoType(t *types.TypeName) (p types.Type) {
	switch t {
	case protoPointer:
		p = &types.Pointer{}
	case protoArray:
		p = &types.Array{}
	case protoSlice:
		p = &types.Slice{}
	case protoMap:
		p = &types.Map{}
	case protoChan:
		p = &types.Chan{Dir: types.SendRecv}
	case protoFunc:
		p = &types.Signature{}
	case protoInterface:
		p = &types.Interface{}
	case protoStruct:
		p = &types.Struct{}
	default:
		panic(fmt.Sprintf("not a proto type %#v", t))
	}
	return
}
