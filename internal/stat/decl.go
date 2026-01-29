package stat

import (
	"go/ast"
	"go/token"
)

// Decls stats about top-level declarations.
type Decls struct {
	Func      int64
	Type      int64
	Interface int64
	Const     int64
	Var       int64
	Other     int64
}

func (s *Decls) Add(b Decls) {
	s.Func += b.Func
	s.Type += b.Type
	s.Interface += b.Interface
	s.Const += b.Const
	s.Var += b.Var
	s.Other += b.Other
}

func (s *Decls) Sub(b Decls) {
	s.Func -= b.Func
	s.Type -= b.Type
	s.Interface -= b.Interface
	s.Const -= b.Const
	s.Var -= b.Var
	s.Other -= b.Other
}

func (s *Decls) Total() int64 {
	return s.Func + s.Type + s.Const + s.Var + s.Other
}

func DeclsFromAst(f *ast.File) Decls {
	stat := Decls{}
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			switch decl.Tok {
			case token.TYPE:
				for _, spec := range decl.Specs {
					stat.Type++
					ts, ok := spec.(*ast.TypeSpec)
					if ok {
						if _, isIface := ts.Type.(*ast.InterfaceType); isIface && ts.Name.IsExported() {
							stat.Interface++
						}
					}
				}
			case token.VAR:
				stat.Var++
			case token.CONST:
				stat.Const++
			default:
				stat.Other++
			}
		case *ast.FuncDecl:
			stat.Func++
		default:
			stat.Other++
		}
	}
	return stat
}
