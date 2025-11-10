package analyzer

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "gometricslint",
	Doc:  "reports forbidden panic and log.Fatal/os.Exit calls outside main.main",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	isMainPkg := pass.Pkg.Name() == "main"

	var funcStack []string
	push := func(name string) { funcStack = append(funcStack, name) }
	pop := func() {
		if len(funcStack) > 0 {
			funcStack = funcStack[:len(funcStack)-1]
		}
	}
	currentFuncName := func() string {
		if len(funcStack) == 0 {
			return ""
		}
		return funcStack[len(funcStack)-1]
	}

	for _, f := range pass.Files {
		astutil.Apply(
			f,
			func(c *astutil.Cursor) bool { // pre
				switch n := c.Node().(type) {
				case *ast.FuncDecl:
					if n.Name != nil {
						push(n.Name.Name)
					}
				case *ast.CallExpr:
					// 1) panic(...)
					if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "panic" {
						if obj := pass.TypesInfo.Uses[ident]; obj == nil {
							pass.Reportf(n.Pos(), "use of panic is forbidden")
						} else if obj.Pkg() == nil && obj.Parent() == types.Universe {
							pass.Reportf(n.Pos(), "use of panic is forbidden")
						}
					}

					// 2) log.Fatal/Fatalf and os.Exit
					if sel, ok := n.Fun.(*ast.SelectorExpr); ok {
						pkgIdent, _ := sel.X.(*ast.Ident)
						if pkgIdent == nil {
							return true
						}
						obj := pass.TypesInfo.Uses[pkgIdent]
						pkgPath := ""
						if obj != nil {
							if pName, ok := obj.(*types.PkgName); ok && pName.Imported() != nil {
								pkgPath = pName.Imported().Path()
							}
						}
						selName := sel.Sel.Name

						forbidden := false
						switch {
						case (pkgPath == "log" || pkgIdent.Name == "log") && (selName == "Fatal" || selName == "Fatalf"):
							forbidden = true
						case (pkgPath == "os" || pkgIdent.Name == "os") && selName == "Exit":
							forbidden = true
						}

						if forbidden {
							fn := currentFuncName()
							if isMainPkg && fn == "main" {
								// Разрешено только в main.main
								return true
							}
							msg := "forbidden outside main.main"
							if strings.HasPrefix(selName, "Fatal") {
								pass.Reportf(n.Pos(), "log.%s %s", selName, msg)
							} else {
								pass.Reportf(n.Pos(), "os.%s %s", selName, msg)
							}
						}
					}
				}
				return true
			},
			func(c *astutil.Cursor) bool { // post
				if _, ok := c.Node().(*ast.FuncDecl); ok {
					pop()
				}
				return true
			},
		)
	}

	return nil, nil
}
