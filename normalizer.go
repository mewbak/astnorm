package astnorm

import (
	"go/ast"
	"go/token"

	"github.com/go-toolsmith/astcast"
	"github.com/go-toolsmith/astequal"
	"github.com/go-toolsmith/typep"
)

type normalizer struct {
	cfg *Config
}

func newNormalizer(cfg *Config) *normalizer {
	return &normalizer{
		cfg: cfg,
	}
}

func (n *normalizer) normalizeExpr(x ast.Expr) ast.Expr {
	switch x := x.(type) {
	case *ast.CallExpr:
		if !typep.IsTypeExpr(n.cfg.Info, x.Fun) {
			x.Fun = n.normalizeExpr(x.Fun)
		}
		x.Args = n.normalizeExprList(x.Args)
		return x
	case *ast.ParenExpr:
		return n.normalizeExpr(x.X)
	default:
		return x
	}
}

func (n *normalizer) normalizeExprList(xs []ast.Expr) []ast.Expr {
	for i, x := range xs {
		xs[i] = n.normalizeExpr(x)
	}
	return xs
}

func (n *normalizer) normalizeStmt(x ast.Stmt) ast.Stmt {
	switch x := x.(type) {
	case *ast.AssignStmt:
		return n.normalizeAssignStmt(x)
	case *ast.BlockStmt:
		return n.normalizeBlockStmt(x)
	default:
		return x
	}
}

func (n *normalizer) normalizeBlockStmt(b *ast.BlockStmt) *ast.BlockStmt {
	// Do multi-stmt transformations before the normalizeStmt loop.
	n.normalizeValSwap(b)

	for i, x := range b.List {
		b.List[i] = n.normalizeStmt(x)
	}
	return b
}

func (n *normalizer) normalizeValSwap(b *ast.BlockStmt) {
	// tmp := x
	// x = y
	// y = tmp
	//
	// =>
	//
	// x, y = y, x
	//
	// FIXME(quasilyte): if tmp is used somewhere outside of the value swap,
	// this transformation is illegal.

	for i := 0; i < len(b.List)-2; i++ {

		assignTmp := astcast.ToAssignStmt(b.List[i+0])
		assignX := astcast.ToAssignStmt(b.List[i+1])
		assignY := astcast.ToAssignStmt(b.List[i+2])
		if assignTmp.Tok != token.DEFINE {
			continue
		}
		if assignX.Tok != token.ASSIGN || assignY.Tok != token.ASSIGN {
			continue
		}
		if len(assignTmp.Lhs) != 1 || len(assignX.Lhs) != 1 || len(assignY.Lhs) != 1 {
			continue
		}
		tmp := astcast.ToIdent(assignTmp.Lhs[0])
		x := assignX.Lhs[0]
		y := assignY.Lhs[0]
		if !astequal.Expr(assignTmp.Rhs[0], x) {
			continue
		}
		if !astequal.Expr(assignX.Rhs[0], y) {
			continue
		}
		if !astequal.Expr(assignY.Rhs[0], tmp) {
			continue
		}

		b.List[i] = &ast.AssignStmt{
			Tok: token.ASSIGN,
			Lhs: []ast.Expr{x, y},
			Rhs: []ast.Expr{y, x},
		}
		b.List = append(b.List[:i+1], b.List[i+3:]...)
	}
}

func (n *normalizer) normalizeAssignStmt(assign *ast.AssignStmt) ast.Stmt {
	assign = n.normalizeAssignOp(assign)
	return assign
}

func (n *normalizer) normalizeAssignOp(assign *ast.AssignStmt) *ast.AssignStmt {
	if assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 {
		return assign
	}
	rhs := astcast.ToBinaryExpr(assign.Rhs[0])
	if !astequal.Expr(assign.Lhs[0], rhs.X) {
		return assign
	}
	// TODO(quasilyte): add missing ops mapping.
	switch rhs.Op {
	case token.ADD:
		assign.Tok = token.ADD_ASSIGN
		assign.Rhs[0] = n.normalizeExpr(rhs.Y)
	}
	return assign
}
