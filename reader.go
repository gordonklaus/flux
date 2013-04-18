package main

import (
	"code.google.com/p/go.exp/go/types"
	"go/ast"
	"go/token"
	"strconv"
)

func loadFunc(f *funcNode) bool {
	file := fluxObjs[f.obj]
	if file == nil {
		return false
	}
	r := &reader{f.obj.GetPkg(), map[string]*types.Package{}, map[string][]*output{}, map[string]types.Type{}}
	for _, i := range file.Imports {
		path, _ := strconv.Unquote(i.Path.Value)
		pkg := r.pkg.Imports[path]
		name := pkg.Name
		if i.Name != nil {
			name = i.Name.Name
		}
		r.pkgNames[name] = pkg
	}
	t := f.obj.GetType().(*types.Signature)
	if t.Recv != nil {
		r.addVar(t.Recv.Name, f.inputsNode.newOutput(t.Recv))
	}
	for _, v := range t.Params {
		r.addVar(v.Name, f.inputsNode.newOutput(v))
		f.addPkgRef(v)
	}
	if d, ok := file.Decls[len(file.Decls)-1].(*ast.FuncDecl); ok {
		r.readBlock(f.funcblk, d.Body.List)
	}
	for _, v := range t.Results {
		r.connect(v.Name, f.outputsNode.newInput(v))
		f.addPkgRef(v)
	}
	return true
}

type reader struct {
	pkg *types.Package
	pkgNames map[string]*types.Package
	vars map[string][]*output // there is a bug here; names can be reused between disjoint blocks; vars should be passed as a param, as in writer
	varTypes map[string]types.Type
}

func (r *reader) readBlock(b *block, s []ast.Stmt) {
	for _, s := range s {
		switch s := s.(type) {
		case *ast.AssignStmt:
			if s.Tok == token.DEFINE {
				switch x := s.Rhs[0].(type) {
				case *ast.BasicLit:
					switch x.Kind {
					case token.INT, token.FLOAT, token.IMAG, token.CHAR:
					case token.STRING:
						n := newStringConstantNode()
						b.addNode(n)
						text, _ := strconv.Unquote(x.Value)
						n.text.SetText(text)
						r.addVar(name(s.Lhs[0]), n.outs[0])
					}
				case *ast.CallExpr:
					n := r.newCallNode(b, x)
					for i, lhs := range s.Lhs {
						r.addVar(name(lhs), n.outs[i])
					}
				case *ast.IndexExpr:
					n := newIndexNode(false)
					b.addNode(n)
					r.connect(name(x.X), n.x)
					r.connect(name(x.Index), n.key)
					r.addVar(name(s.Lhs[0]), n.outVal)
					if len(s.Lhs) == 2 {
						r.addVar(name(s.Lhs[1]), n.ok)
					}
				}
			} else {
				if x, ok := s.Lhs[0].(*ast.IndexExpr); ok {
					n := newIndexNode(true)
					b.addNode(n)
					r.connect(name(x.X), n.x)
					r.connect(name(x.Index), n.key)
					if i, ok := s.Rhs[0].(*ast.Ident); ok {
						r.connect(i.Name, n.inVal)
					}
				} else {
					for i := range s.Lhs {
						lh := name(s.Lhs[i])
						rh := name(s.Rhs[i])
						r.vars[lh] = append(r.vars[lh], r.vars[rh]...)
						// the static type of lhs and rhs are not necessarily the same.
						// varType is set under DeclStmt.
						// until go/types is complete, setting varType here, which will work in most cases but fail in a few
						r.varTypes[lh] = r.varTypes[rh]
					}
				}
			}
		case *ast.DeclStmt:
			v := s.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
			// this only handles named types; when will go/types do it all for me?
			var t types.Type
			switch x := v.Type.(type) {
			case *ast.Ident:
				t = r.pkg.Scope.Lookup(x.Name).(*types.TypeName).Type
			case *ast.SelectorExpr:
				t = r.pkgNames[name(x.X)].Scope.Lookup(x.Sel.Name).(*types.TypeName).Type
			}
			if t != nil {
				r.varTypes[v.Names[0].Name] = t
			}
		case *ast.ForStmt:
			n := newLoopNode()
			b.addNode(n)
			if s.Cond != nil {
				r.connect(name(s.Cond.(*ast.BinaryExpr).Y), n.input)
			}
			if s.Init != nil {
				r.addVar(name(s.Init.(*ast.AssignStmt).Lhs[0]), n.inputsNode.outs[0])
			}
			r.readBlock(n.loopblk, s.Body.List)
		case *ast.IfStmt:
			n := newIfNode()
			b.addNode(n)
			r.connect(name(s.Cond), n.input)
			r.readBlock(n.trueblk, s.Body.List)
			if s.Else != nil {
				r.readBlock(n.falseblk, s.Else.(*ast.BlockStmt).List)
			}
		case *ast.RangeStmt:
			n := newLoopNode()
			b.addNode(n)
			r.connect(name(s.X), n.input)
			r.addVar(name(s.Key), n.inputsNode.outs[0])
			if s.Value != nil {
				r.addVar(name(s.Value), n.inputsNode.outs[1])
			}
			r.readBlock(n.loopblk, s.Body.List)
		case *ast.ExprStmt:
			switch x := s.X.(type) {
			case *ast.CallExpr:
				r.newCallNode(b, x)
			}
		}
	}
}

func (r *reader) newCallNode(b *block, x *ast.CallExpr) (n *callNode) {
	var recvExpr ast.Expr
	switch f := x.Fun.(type) {
	case *ast.Ident:
		n = newCallNode(r.pkg.Scope.Lookup(f.Name))
	case *ast.SelectorExpr:
		n1 := name(f.X)
		n2 := f.Sel.Name
		if pkg, ok := r.pkgNames[n1]; ok {
			n = newCallNode(pkg.Scope.Lookup(n2))
		} else {
			recv := r.varTypes[n1]
			if p, ok := recv.(*types.Pointer); ok {
				recv = p.Base
			}
			for _, m := range recv.(*types.NamedType).Methods {
				if m.Name == n2 {
					n = newCallNode(method{nil, m})
					break
				}
			}
			recvExpr = f.X
		}
	}
	b.addNode(n)
	args := x.Args
	if recvExpr != nil {
		args = append([]ast.Expr{recvExpr}, args...)
	}
	for i, arg := range args {
		if arg, ok := arg.(*ast.Ident); ok {
			r.connect(arg.Name, n.ins[i])
		}
	}
	return
}

func (r *reader) connect(name string, in *input) {
	for _, out := range r.vars[name] {
		c := newConnection()
		c.setSrc(out)
		c.setDst(in)
	}
}

func (r *reader) addVar(name string, out *output) {
	if name != "" && name != "_" {
		r.vars[name] = append(r.vars[name], out)
		r.varTypes[name] = out.obj.GetType()
	}
}

func name(x ast.Expr) string {
	return x.(*ast.Ident).Name
}
