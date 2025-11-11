package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Маркер над структурой
const marker = "generate:reset"

type targetStruct struct {
	TypeName string        // имя типа
	Named    *types.Named  // *types.Named типа
	Struct   *types.Struct // подложка
	PkgPath  string        // импортный путь пакета
	PkgDir   string        // директория пакета (куда писать reset.gen.go)
}

// --- загрузка пакетов и поиск помеченных структур ---

func main() {
	if err := run(); err != nil {
		// ошибка в нижнем регистре — для ST1005
		fmt.Fprintf(os.Stderr, "generator error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root := "."
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedModule,
		Dir: root,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return errors.New("no packages found")
	}
	if packages.PrintErrors(pkgs) > 0 {
		// не выходим — попробуем сгенерить то, что можем
	}

	// пакет -> список структур для генерации
	perPkg := map[string][]*targetStruct{} // key = pkg.PkgPath
	perPkgDir := map[string]string{}       // pkgPath -> directory
	perPkgName := map[string]string{}      // pkgPath -> package name
	seen := map[string]struct{}{}          // защита от дублирования типов
	fset := token.NewFileSet()             // лишь для форматирования позиций (необяз.)
	_ = fset

	for _, pkg := range pkgs {
		if pkg.PkgPath == "" || len(pkg.Syntax) == 0 || pkg.Types == nil {
			continue
		}
		// пропускаем тестовые-надпакеты "xxx [xxx.test]"
		if strings.HasSuffix(pkg.PkgPath, ".test") {
			continue
		}

		// определим директорию пакета (берем у первого файла)
		pkgDir := ""
		if len(pkg.GoFiles) > 0 {
			pkgDir = filepath.Dir(pkg.GoFiles[0])
		}
		if pkgDir == "" {
			continue
		}
		perPkgDir[pkg.PkgPath] = pkgDir
		perPkgName[pkg.PkgPath] = pkg.Name

		// пройдем все файлы (AST) и найдем struct с комментарием-маркером
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				gen, ok := n.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					return true
				}

				// проверим комментарии около объявления (Doc у GenDecl и Doc у TypeSpec)
				genHasMarker := commentGroupHasMarker(gen.Doc, marker)

				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, ok := ts.Type.(*ast.StructType); !ok {
						continue
					}

					hasMarker := genHasMarker || commentGroupHasMarker(ts.Doc, marker) || hasInlineMarker(file, ts)

					if !hasMarker {
						continue
					}

					// достанем тип из go/types по имени
					obj := pkg.Types.Scope().Lookup(ts.Name.Name)
					if obj == nil {
						continue
					}
					named, ok := obj.Type().(*types.Named)
					if !ok {
						continue
					}
					under, ok := named.Underlying().(*types.Struct)
					if !ok {
						continue
					}

					key := pkg.PkgPath + "." + ts.Name.Name
					if _, exists := seen[key]; exists {
						continue
					}
					seen[key] = struct{}{}

					perPkg[pkg.PkgPath] = append(perPkg[pkg.PkgPath], &targetStruct{
						TypeName: ts.Name.Name,
						Named:    named,
						Struct:   under,
						PkgPath:  pkg.PkgPath,
						PkgDir:   pkgDir,
					})
				}
				return false // не углубляемся внутрь объявления типа
			})
		}
	}

	// генерируем по пакетам reset.gen.go
	for pkgPath, structs := range perPkg {
		sort.Slice(structs, func(i, j int) bool { return structs[i].TypeName < structs[j].TypeName })
		pkgName := perPkgName[pkgPath]
		out, err := generateForPackage(pkgName, structs)
		if err != nil {
			return fmt.Errorf("package %s: %w", pkgPath, err)
		}

		dir := perPkgDir[pkgPath]
		filename := filepath.Join(dir, "reset.gen.go")
		if err := os.WriteFile(filename, out, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
		fmt.Printf("generated %s (%d structs)\n", filename, len(structs))
	}

	return nil
}

func commentGroupHasMarker(cg *ast.CommentGroup, m string) bool {
	if cg == nil {
		return false
	}
	for _, c := range cg.List {
		if strings.Contains(c.Text, m) {
			return true
		}
	}
	return false
}

// Иногда комментарий ставят хвостовым // generate:reset на той же строке.
// Попробуем найти строчный комментарий рядом с именем типа.
func hasInlineMarker(file *ast.File, ts *ast.TypeSpec) bool {
	if file == nil || ts == nil {
		return false
	}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			// ограничимся грубым проверочным правилом — комментарий на той же строке.
			if file.Pos() <= c.Pos() && c.Pos() <= file.End() {
				if fileset := file.Pos(); fileset != token.NoPos {
					// упрощаем: просто ищем маркер в тексте, это дешевле и надежно
					if strings.Contains(c.Text, marker) {
						return true
					}
				}
			}
		}
	}
	return false
}

// --- генерация исходника пакета ---

func generateForPackage(pkgName string, structs []*targetStruct) ([]byte, error) {
	var buf bytes.Buffer

	fmt.Fprintln(&buf, "// Code generated by reset generator; DO NOT EDIT.")
	fmt.Fprintln(&buf, "// generated by cmd/reset")
	fmt.Fprintf(&buf, "package %s\n\n", pkgName)

	for _, ts := range structs {
		code := generateResetMethod(ts)
		buf.WriteString(code)
		buf.WriteByte('\n')
	}

	// форматирование
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// если вдруг где-то промах по синтаксису — вернем неформатированный для диагностики
		return buf.Bytes(), nil
	}
	return formatted, nil
}

func generateResetMethod(ts *targetStruct) string {
	var b strings.Builder

	recv := "r"
	typeName := ts.TypeName

	fmt.Fprintf(&b, "func (%s *%s) Reset() {\n", recv, typeName)
	fmt.Fprintf(&b, "    if %s == nil {\n        return\n    }\n\n", recv)

	st := ts.Struct
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		// Имя поля. Для embedded без имени используем его имя типа.
		fieldName := f.Name()
		if fieldName == "" {
			// теоретически не бывает, но на всякий
			fieldName = embeddedFieldName(f.Type())
		}
		fieldExpr := recv + "." + fieldName
		b.WriteString(genResetForField(fieldExpr, f.Type()))
	}

	fmt.Fprintln(&b, "}")
	return b.String()
}

func embeddedFieldName(t types.Type) string {
	switch tt := t.(type) {
	case *types.Named:
		return tt.Obj().Name()
	default:
		return "/*embedded*/"
	}
}

// Генерация кода сброса для одного поля
func genResetForField(lvalue string, t types.Type) string {
	switch tt := t.(type) {
	case *types.Basic:
		return fmt.Sprintf("    %s = %s\n", lvalue, zeroLiteralForBasic(tt))
	case *types.Slice:
		// слайс -> [:0], но так, чтобы nil не паниковал
		return fmt.Sprintf("    if %s != nil { %s = %s[:0] }\n", lvalue, lvalue, lvalue)
	case *types.Map:
		// clear работает и на nil
		return fmt.Sprintf("    clear(%s)\n", lvalue)
	case *types.Pointer:
		// указатель не зануляем, но сбрасываем то, на что он указывает
		var sb strings.Builder
		fmt.Fprintf(&sb, "    if %s != nil {\n", lvalue)
		sb.WriteString(genResetPointed("*"+lvalue, tt.Elem()))
		fmt.Fprintf(&sb, "    }\n")
		return sb.String()
	case *types.Named:
		under := tt.Underlying()
		// если это структура и у нее есть метод Reset — вызвать
		if _, ok := under.(*types.Struct); ok && hasResetMethod(tt) {
			// поле-значение: (&field).Reset()
			return fmt.Sprintf("    (&%s).Reset()\n", lvalue)
		}
		// иначе — попробовать обработать по подложке (например, alias на слайс/мапу/бейсик)
		return genResetForField(lvalue, under)
	case *types.Struct:
		// у безымянной структуры Reset не предполагается — ничего не делаем (по ТЗ не требуется нулить)
		// но попробуем позвать Reset, если вдруг метод есть как у значения (редкий случай)
		if hasResetMethod(tt) {
			return fmt.Sprintf("    (&%s).Reset()\n", lvalue)
		}
		return ""
	case *types.Interface:
		// если интерфейсная переменная содержит тип с Reset, дернем через type assertion
		return fmt.Sprintf("    if v, ok := any(%s).(interface{ Reset() }); ok && v != nil { v.Reset() }\n", lvalue)
	default:
		// channel, func, array, etc. — сведем к нулю, если возможно
		switch t.(type) {
		case *types.Chan, *types.Signature:
			return fmt.Sprintf("    %s = nil\n", lvalue)
		case *types.Array:
			// присвоим ноль всего массива: var = [N]T{}
			return fmt.Sprintf("    %s = %s{}\n", lvalue, types.TypeString(t, nil))
		default:
			return ""
		}
	}
}

func genResetPointed(derefLValue string, elem types.Type) string {
	// derefLValue = "*r.Field"
	switch et := elem.(type) {
	case *types.Basic:
		return fmt.Sprintf("        %s = %s\n", derefLValue, zeroLiteralForBasic(et))
	case *types.Slice:
		// (*p) = (*p)[:0] — но аккуратно с nil
		return fmt.Sprintf("        if %s != nil { %s = %s[:0] }\n", derefLValue, derefLValue, derefLValue)
	case *types.Map:
		return fmt.Sprintf("        clear(%s)\n", derefLValue)
	case *types.Pointer:
		// указатель на указатель — рекурсивно
		var sb strings.Builder
		fmt.Fprintf(&sb, "        if %s != nil {\n", derefLValue)
		sb.WriteString(genResetPointed("*"+derefLValue, et.Elem()))
		fmt.Fprintf(&sb, "        }\n")
		return sb.String()
	case *types.Named:
		under := et.Underlying()
		// если у целевого типа есть Reset — вызовем
		if hasResetMethod(et) {
			// (*p).Reset()
			return fmt.Sprintf("        (%s).Reset()\n", derefLValue)
		}
		// иначе обработаем по подложке (alias на базовый/срез/мапу/структуру)
		return indent(genResetPointed(derefLValue, under), 0)
	case *types.Struct:
		// если на структуре есть Reset — вызовем
		if hasResetMethod(elem) {
			return fmt.Sprintf("        (%s).Reset()\n", derefLValue)
		}
		// иначе — ничего (по ТЗ нулить вложенную структуру не обязательно)
		return ""
	case *types.Interface:
		// *iface — крайне редкий случай, но корректно: сначала проверить не nil интерфейс, затем дернуть Reset
		return fmt.Sprintf("        if v, ok := any(%s).(interface{ Reset() }); ok && v != nil { v.Reset() }\n", derefLValue)
	default:
		switch elem.(type) {
		case *types.Chan, *types.Signature:
			return fmt.Sprintf("        %s = nil\n", derefLValue)
		case *types.Array:
			return fmt.Sprintf("        %s = %s{}\n", derefLValue, types.TypeString(elem, nil))
		default:
			return ""
		}
	}
}

func zeroLiteralForBasic(b *types.Basic) string {
	switch b.Kind() {
	case types.Bool:
		return "false"
	case types.String:
		return `""`
	// все числовые сведём к 0, включая rune/byte/complex/float
	case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
		types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64, types.Uintptr,
		types.Float32, types.Float64,
		types.Complex64, types.Complex128,
		types.UntypedInt, types.UntypedFloat, types.UntypedComplex, types.UntypedRune:
		return "0"
	case types.UntypedBool, types.UntypedString:
		return "0" // до такого не дойдет на реальных полях
	default:
		return "0"
	}
}

// Проверяет наличие метода Reset у типа (значение или указатель).
func hasResetMethod(t types.Type) bool {
	// Смотрим методсет указателя — он включает методы и с value-, и с pointer-ресивером.
	ptr := types.NewPointer(t)
	ms := types.NewMethodSet(ptr)
	for i := 0; i < ms.Len(); i++ {
		if ms.At(i).Obj().Name() == "Reset" {
			return true
		}
	}
	// На всякий — проверим и value-set
	ms2 := types.NewMethodSet(t)
	for i := 0; i < ms2.Len(); i++ {
		if ms2.At(i).Obj().Name() == "Reset" {
			return true
		}
	}
	return false
}

func indent(s string, n int) string {
	if s == "" {
		return s
	}
	pad := strings.Repeat(" ", n)
	var out strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		out.WriteString(pad)
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
