package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var Analyzer = &analysis.Analyzer{
	Name: "fieldcheck",
	Doc:  "check model json/gorm tag consistency and dataaccess condition field names",
	Run:  run,
}

// fieldConfig 描述如何从函数调用中提取字段名
type fieldConfig struct {
	argIndexes []int // 哪些参数位置是字段名（0-based）
	allStrings bool  // true = 所有 string 字面量参数都算字段
}

var fieldFuncs = map[string]fieldConfig{
	// 条件函数 - 第一个参数是字段名
	"Eq":            {argIndexes: []int{0}},
	"IfEq":          {argIndexes: []int{0}},
	"Ne":            {argIndexes: []int{0}},
	"IfNe":          {argIndexes: []int{0}},
	"Gt":            {argIndexes: []int{0}},
	"IfGt":          {argIndexes: []int{0}},
	"Gte":           {argIndexes: []int{0}},
	"IfGte":         {argIndexes: []int{0}},
	"Lt":            {argIndexes: []int{0}},
	"IfLt":          {argIndexes: []int{0}},
	"Lte":           {argIndexes: []int{0}},
	"IfLte":         {argIndexes: []int{0}},
	"In":            {argIndexes: []int{0}},
	"IfIn":          {argIndexes: []int{0}},
	"NotIn":         {argIndexes: []int{0}},
	"IfNotIn":       {argIndexes: []int{0}},
	"Like":          {argIndexes: []int{0}},
	"IfLike":        {argIndexes: []int{0}},
	"NotLike":       {argIndexes: []int{0}},
	"IfNotLike":     {argIndexes: []int{0}},
	"Between":       {argIndexes: []int{0}},
	"IfBetween":     {argIndexes: []int{0}},
	"NotBetween":    {argIndexes: []int{0}},
	"IfNotBetween":  {argIndexes: []int{0}},
	"IsNull":        {argIndexes: []int{0}},
	"IsNotNull":     {argIndexes: []int{0}},
	// 排序 - 第一个参数是字段名
	"Order":      {argIndexes: []int{0}},
	// JOIN - 两个参数都是字段名
	"JoinOn":     {argIndexes: []int{0, 1}},
	// SelectFunc - SF(field), SFN(n, field)
	"SF":         {argIndexes: []int{0}},
	"SFN":        {argIndexes: []int{1}},
	// OrderCase - 第一个参数是字段名
	"OrderCase":     {argIndexes: []int{0}},
	"OrderCaseDesc": {argIndexes: []int{0}},
	// Distinct / OnlyFields - 所有 string 参数都是字段名
	"Distinct":   {allStrings: true},
	"OnlyFields": {allStrings: true},
}

var accessorMethods = map[string]bool{
	"List": true, "First": true, "Count": true,
	"Get": true, "Update": true, "Delete": true, "Create": true,
}

func run(pass *analysis.Pass) (interface{}, error) {
	modelFields := make(map[string]map[string]bool)

	// 第一步：检查 model 自身 json/gorm tag 一致性
	checkModelTagConsistency(pass, modelFields)

	if len(modelFields) == 0 {
		return nil, nil
	}

	// 第二步：检查所有引用字段名是否在 model 中
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			fields := extractFields(call)
			if len(fields) == 0 {
				return true
			}

			modelType := resolveModelFromTypesInfo(pass, call)

			for _, ef := range fields {
				fieldName := ef.Name
				if strings.ContainsAny(fieldName, "(, ") {
					continue
				}

				if modelType != "" {
					if mf, ok := modelFields[modelType]; ok {
						if !mf[fieldName] {
							suggest := suggestField(fieldName, mf)
							pass.Report(analysis.Diagnostic{
								Pos:     ef.Pos,
								Message: fmt.Sprintf("fieldcheck: 字段 %q 在模型 %s 中不存在%s", fieldName, modelType, suggest),
							})
						}
						continue
					}
				}

				// 退化：检查所有 model 字段并集
				found := false
				for _, mf := range modelFields {
					if mf[fieldName] {
						found = true
						break
					}
				}
				if !found {
					allFields := make(map[string]bool)
					for _, mf := range modelFields {
						for f := range mf {
							allFields[f] = true
						}
					}
					suggest := suggestField(fieldName, allFields)
					pass.Report(analysis.Diagnostic{
						Pos:     ef.Pos,
						Message: fmt.Sprintf("fieldcheck: 字段 %q 在所有模型中都不存在%s", fieldName, suggest),
					})
				}
			}

			return true
		})
	}

	return nil, nil
}

// ============================================================
// 第一步：检查 model json/gorm tag 一致性
// ============================================================

func checkModelTagConsistency(pass *analysis.Pass, result map[string]map[string]bool) {
	checked := make(map[string]bool)

	for _, obj := range pass.TypesInfo.Defs {
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		checkNamedModel(pass, named, result, checked)
	}

	for _, obj := range pass.TypesInfo.Uses {
		if obj == nil {
			continue
		}
		if tn, ok := obj.(*types.TypeName); ok {
			if named, ok := tn.Type().(*types.Named); ok {
				checkNamedModel(pass, named, result, checked)
			}
		}
	}
}

func checkNamedModel(pass *analysis.Pass, named *types.Named, result map[string]map[string]bool, checked map[string]bool) {
	obj := named.Obj()
	if obj == nil {
		return
	}
	typeName := obj.Name()
	pkg := obj.Pkg()

	key := typeName
	if pkg != nil {
		key = pkg.Path() + "." + typeName
	}
	if checked[key] {
		return
	}
	checked[key] = true

	if !hasTableNameMethod(named) {
		return
	}

	// 检查 tag 一致性并收集字段
	fields := make(map[string]bool)
	checkStructTags(pass, named, fields)

	if len(fields) > 0 {
		result[typeName] = fields
		if pkg != nil {
			result[pkg.Path()+"."+typeName] = fields
		}
	}
}

// checkStructTags 检查 model struct 中每个字段的 json 和 gorm tag 是否一致
func checkStructTags(pass *analysis.Pass, named *types.Named, fields map[string]bool) {
	strct, ok := named.Underlying().(*types.Struct)
	if !ok {
		return
	}

	for i := 0; i < strct.NumFields(); i++ {
		f := strct.Field(i)

		if f.Embedded() {
			if embeddedNamed := resolveNamedType(f.Type()); embeddedNamed != nil {
				checkStructTags(pass, embeddedNamed, fields)
			}
			continue
		}

		if !f.Exported() {
			continue
		}

		fields[f.Name()] = true

		tag := strct.Tag(i)
		if tag == "" {
			continue
		}

		jsonName := ""
		gormName := ""

		if jt := lookupTag(tag, "json"); jt != "" {
			jsonName = strings.Split(jt, ",")[0]
			if jsonName != "" && jsonName != "-" {
				fields[jsonName] = true
			}
		}

		if gt := lookupTag(tag, "gorm"); gt != "" {
			gormName = extractGormColumn(gt)
			if gormName != "" {
				fields[gormName] = true
			}
		}

		// 一致性检查：json 和 gorm 都存在时，直接比较值是否相同
		if jsonName != "" && gormName != "" {
			if jsonName != gormName {
				pos := findFieldPos(pass, named, f.Name())
				if pos != token.NoPos {
					pass.Report(analysis.Diagnostic{
						Pos:     pos,
						Message: fmt.Sprintf("fieldcheck: %s.%s 的 json 标签 %q 和 gorm 列名 %q 不一致，json 应改为 %q", named.Obj().Name(), f.Name(), jsonName, gormName, gormName),
					})
				}
			}
		}
	}
}

// findFieldPos 找到 struct field 的 AST 位置
func findFieldPos(pass *analysis.Pass, named *types.Named, fieldName string) token.Pos {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil {
					continue
				}
				obj := pass.TypesInfo.Defs[typeSpec.Name]
				if obj == nil {
					continue
				}
				if obj.Type() != named && obj.Type().(*types.Named) != named {
					// 比较类型名
					n, ok := obj.Type().(*types.Named)
					if !ok || n.Obj().Name() != named.Obj().Name() {
						continue
					}
					if n.Obj().Pkg() != nil && named.Obj().Pkg() != nil {
						if n.Obj().Pkg().Path() != named.Obj().Pkg().Path() {
							continue
						}
					}
				}

				strct, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}

				for _, field := range strct.Fields.List {
					for _, name := range field.Names {
						if name.Name == fieldName {
							return name.Pos()
						}
					}
				}
			}
		}
	}
	return token.NoPos
}

// ============================================================
// 通用工具函数
// ============================================================

func resolveNamedType(t types.Type) *types.Named {
	switch v := t.(type) {
	case *types.Named:
		return v
	case *types.Pointer:
		return resolveNamedType(v.Elem())
	}
	return nil
}

func hasTableNameMethod(named *types.Named) bool {
	for i := 0; i < named.NumMethods(); i++ {
		m := named.Method(i)
		if m.Name() == "TableName" {
			sig, ok := m.Type().(*types.Signature)
			if !ok {
				continue
			}
			if sig.Params().Len() == 0 && sig.Results().Len() == 1 {
				if basic, ok := sig.Results().At(0).Type().(*types.Basic); ok {
					return basic.Kind() == types.String
				}
			}
		}
	}
	return false
}

func lookupTag(tagStr, key string) string {
	for tagStr != "" {
		i := strings.Index(tagStr, ":")
		if i < 0 {
			break
		}
		tagKey := strings.TrimSpace(tagStr[:i])
		tagStr = tagStr[i+1:]
		if tagStr == "" || tagStr[0] != '"' {
			break
		}
		end := strings.Index(tagStr[1:], "\"")
		if end < 0 {
			break
		}
		tagValue := tagStr[1 : end+1]
		tagStr = tagStr[end+2:]
		if tagKey == key {
			return tagValue
		}
	}
	return ""
}

func extractGormColumn(gormTag string) string {
	for _, part := range strings.Split(gormTag, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "column:") {
			return strings.TrimPrefix(part, "column:")
		}
	}
	gormTag = strings.TrimSpace(gormTag)
	if idx := strings.Index(gormTag, ";"); idx >= 0 {
		gormTag = gormTag[:idx]
	}
	if idx := strings.Index(gormTag, ":"); idx >= 0 {
		return ""
	}
	gormTag = strings.TrimSpace(gormTag)
	if gormTag != "" && !strings.Contains(gormTag, ":") && !strings.Contains(gormTag, "(") {
		return gormTag
	}
	return ""
}

// ============================================================
// 第二步：通过 TypesInfo 解析 model 类型
// ============================================================

func resolveModelFromTypesInfo(pass *analysis.Pass, conditionCall *ast.CallExpr) string {
	// 通过 Selections 查找
	for expr, sel := range pass.TypesInfo.Selections {
		if sel == nil {
			continue
		}
		methodName := sel.Obj().Name()
		if !accessorMethods[methodName] {
			continue
		}
		recv := sel.Recv()
		modelName := extractModelFromRecvType(recv)
		if modelName == "" {
			continue
		}
		selCall, ok := findParentCallExpr(pass, expr)
		if !ok {
			continue
		}
		if isArgOf(conditionCall, selCall) {
			return modelName
		}
	}

	// 退化：扫描 AST
	for _, file := range pass.Files {
		var found string
		ast.Inspect(file, func(n ast.Node) bool {
			if found != "" {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			selExpr, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if !accessorMethods[selExpr.Sel.Name] {
				return true
			}
			if !isArgOf(conditionCall, call) {
				return true
			}
			recvType := pass.TypesInfo.TypeOf(selExpr.X)
			if recvType == nil {
				return true
			}
			if modelName := extractModelFromRecvType(recvType); modelName != "" {
				found = modelName
			}
			return true
		})
		if found != "" {
			return found
		}
	}

	return ""
}

func extractModelFromRecvType(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		return extractModelFromRecvType(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok {
		return ""
	}
	objName := named.Obj().Name()
	if objName != "DataAccessor" && objName != "KvDBDataAccessor" && objName != "GormDataAccessor" {
		return ""
	}
	if named.TypeArgs().Len() != 1 {
		return ""
	}
	arg := named.TypeArgs().At(0)
	if ptr, ok := arg.(*types.Pointer); ok {
		arg = ptr.Elem()
	}
	if argNamed, ok := arg.(*types.Named); ok {
		return argNamed.Obj().Name()
	}
	return ""
}

func findParentCallExpr(pass *analysis.Pass, selExpr *ast.SelectorExpr) (*ast.CallExpr, bool) {
	for _, file := range pass.Files {
		var result *ast.CallExpr
		ast.Inspect(file, func(n ast.Node) bool {
			if result != nil {
				return false
			}
			if call, ok := n.(*ast.CallExpr); ok {
				if call.Fun == selExpr {
					result = call
					return false
				}
			}
			return true
		})
		if result != nil {
			return result, true
		}
	}
	return nil, false
}

func isArgOf(target, parent *ast.CallExpr) bool {
	for _, arg := range parent.Args {
		if arg == target {
			return true
		}
		if nestedCall, ok := arg.(*ast.CallExpr); ok {
			if nestedCall == target {
				return true
			}
		}
	}
	return false
}

// ============================================================
// 字段提取 & 建议
// ============================================================

type extractedField struct {
	Name string
	Pos  token.Pos
}

func extractFields(call *ast.CallExpr) []extractedField {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	cfg, ok := fieldFuncs[sel.Sel.Name]
	if !ok {
		return nil
	}

	var fields []extractedField

	if cfg.allStrings {
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				f := strings.Trim(lit.Value, "\"`")
				if f != "" {
					fields = append(fields, extractedField{Name: f, Pos: lit.Pos()})
				}
			}
		}
	} else {
		for _, idx := range cfg.argIndexes {
			if idx >= len(call.Args) {
				continue
			}
			lit, ok := call.Args[idx].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			f := strings.Trim(lit.Value, "\"`")
			if f != "" {
				fields = append(fields, extractedField{Name: f, Pos: lit.Pos()})
			}
		}
	}

	return fields
}

func suggestField(field string, validFields map[string]bool) string {
	best := ""
	bestDist := len(field) + 1
	for f := range validFields {
		d := levenshtein(field, f)
		if d < bestDist {
			bestDist = d
			best = f
		}
	}
	if best != "" && bestDist <= 3 {
		return fmt.Sprintf("，是否应该是 %q？", best)
	}
	return ""
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func main() {
	singlechecker.Main(Analyzer)
}
