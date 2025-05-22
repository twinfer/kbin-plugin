package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"unicode"
)

const (
	ksyDirRelative        = "test/formats"
	binDirRelative        = "test/src"
	kscGoTestDirRelative  = "test/go" // For KSC Go test files
	kscGenDirRelative     = "testdata/formats_kaitai_go_gen"
	testOutputDirRelative = "testdata/kaitaistruct/formats_test"
	testTemplateFile      = "scripts/test_template.tmpl" // Path relative to project root
)

type TemplateData struct {
	FormatName         string          // e.g., "png", "bits_simple"
	StructName         string          // e.g., "Png", "BitsSimple" (PascalCase)
	KsyFileName        string          // e.g., "png.ksy"
	FormatPackageAlias string          // e.g., "png_kaitai"
	TestCases          []TestCaseData  // For .bin file based tests
	Assertions         []AssertionData // For assertions from KSC tests
}

type TestCaseData struct {
	Name           string // e.g., "sample1" (from sample1.bin)
	BinFileRelPath string // Relative path of .bin file
}

type AssertionData struct {
	ExpectedValueRaw  string   // Raw string of the expected value, e.g., "uint32(123)", "[]uint8{1,2}", "3"
	BaseObjectPath    []string // Path to the base object in customMap, e.g., for r.Ltr.AsInt -> ["Ltr", "AsInt"]; for r.ArrayOfInts[0] -> ["ArrayOfInts"]
	Operation         string   // Type of operation on BaseObjectPath: "" (direct), "INDEX", "LEN", "GETTER"
	OperationArg      string   // Argument for the operation, e.g., "0" for INDEX
	OriginalKscGoExpr string   // The original KSC Go expression for the actual value, for reference
}

func getProjectRoot() string {
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Failed to get caller information to determine script directory")
	}
	scriptDir := filepath.Dir(b)
	return filepath.Clean(filepath.Join(scriptDir, ".."))
}

func main() {
	projectRoot := getProjectRoot()

	absKsyDir := filepath.Join(projectRoot, ksyDirRelative)
	absBinDir := filepath.Join(projectRoot, binDirRelative)
	absKscGoTestDir := filepath.Join(projectRoot, kscGoTestDirRelative)
	absKscGenDir := filepath.Join(projectRoot, kscGenDirRelative)
	absTestOutputDir := filepath.Join(projectRoot, testOutputDirRelative)
	absTestTemplateFile := filepath.Join(projectRoot, testTemplateFile)

	log.Printf("Project Root: %s", projectRoot)
	log.Printf("KSY Source Directory: %s", absKsyDir)
	log.Printf("BIN Source Directory: %s", absBinDir)
	log.Printf("KSC Go Test Directory: %s", absKscGoTestDir)
	log.Printf("KSC Gen Directory: %s", absKscGenDir)
	log.Printf("Test Output Directory: %s", absTestOutputDir)
	log.Printf("Test Template: %s", absTestTemplateFile)

	if err := os.MkdirAll(absKscGenDir, 0755); err != nil {
		log.Fatalf("Failed to create KSC gen dir '%s': %v", absKscGenDir, err)
	}
	if err := os.MkdirAll(absTestOutputDir, 0755); err != nil {
		log.Fatalf("Failed to create test output dir '%s': %v", absTestOutputDir, err)
	}

	tmpl, err := template.New(filepath.Base(absTestTemplateFile)).
		Funcs(template.FuncMap{"getMapKeys": func(m map[string]any) []string {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			return keys
		}}).
		ParseFiles(absTestTemplateFile)
	if err != nil {
		log.Fatalf("Failed to parse template '%s': %v", absTestTemplateFile, err)
	}

	ksyFiles, err := filepath.Glob(filepath.Join(absKsyDir, "*.ksy"))
	if err != nil {
		log.Fatalf("Failed to glob KSY files in '%s': %v", absKsyDir, err)
	}

	for _, ksyFilePath := range ksyFiles {
		ksyFileName := filepath.Base(ksyFilePath)
		formatName := strings.TrimSuffix(ksyFileName, ".ksy")
		goPackageName := strings.ReplaceAll(strings.ReplaceAll(formatName, "-", "_"), ".", "_")

		log.Printf("Processing format: %s (KSY: %s)", formatName, ksyFileName)

		cmd := exec.Command("ksc", "-t", "go", "--outdir", absKscGenDir, "--go-package", goPackageName, "--import-path", absKsyDir, ksyFilePath)
		var kscErr bytes.Buffer
		cmd.Stderr = &kscErr
		if err := cmd.Run(); err != nil {
			log.Printf("KSC failed for %s (KSY: %s): %v. Stderr: %s. Skipping.", formatName, ksyFilePath, err, kscErr.String())
			continue
		}

		var testCases []TestCaseData
		binFilePath := filepath.Join(absBinDir, formatName+".bin")
		if _, err := os.Stat(binFilePath); err == nil {
			testCases = append(testCases, TestCaseData{
				Name:           formatName,
				BinFileRelPath: formatName + ".bin",
			})
		} else {
			log.Printf("No .bin sample file found for %s at %s.", formatName, binFilePath)
		}

		kscGoTestFilePath := filepath.Join(absKscGoTestDir, goPackageName+"_test.go")
		assertions := extractAssertionsFromKscTest(kscGoTestFilePath)

		if len(testCases) == 0 && len(assertions) == 0 {
			log.Printf("No .bin samples and no assertions found for %s. Skipping test file generation.", formatName)
			continue
		}

		data := TemplateData{
			FormatName:         formatName,
			StructName:         toPascalCase(goPackageName),
			KsyFileName:        ksyFileName,
			FormatPackageAlias: goPackageName + "_kaitai",
			TestCases:          testCases,
			Assertions:         assertions,
		}

		outputTestFilePath := filepath.Join(absTestOutputDir, goPackageName+"_gen_test.go")
		outFile, err := os.Create(outputTestFilePath)
		if err != nil {
			log.Fatalf("Failed to create test file %s: %v", outputTestFilePath, err)
		}

		var renderedTest bytes.Buffer
		if err := tmpl.Execute(&renderedTest, data); err != nil {
			outFile.Close()
			log.Fatalf("Failed to execute template for %s: %v", formatName, err)
		}

		formattedBytes, err := formatSource(outputTestFilePath, renderedTest.Bytes())
		if err != nil {
			log.Printf("Warning: goimports failed for %s: %v. Writing unformatted code.", outputTestFilePath, err)
			if _, writeErr := outFile.Write(renderedTest.Bytes()); writeErr != nil {
				log.Printf("Failed to write unformatted code to %s: %v", outputTestFilePath, writeErr)
			}
		} else {
			if _, writeErr := outFile.Write(formattedBytes); writeErr != nil {
				log.Printf("Failed to write formatted code to %s: %v", outputTestFilePath, writeErr)
			}
		}
		outFile.Close()
		log.Printf("Generated test file: %s", outputTestFilePath)
	}
	log.Println("Test generation script finished.")
}

func toPascalCase(s string) string {
	var result strings.Builder
	capitalizeNext := true
	for _, r := range s {
		if r == '_' || r == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func formatSource(filename string, src []byte) ([]byte, error) {
	cmd := exec.Command("goimports", "-srcdir", filepath.Dir(filename))
	cmd.Stdin = bytes.NewReader(src)
	var out bytes.Buffer
	cmd.Stdout = &out
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return src, fmt.Errorf("goimports failed for %s: %w\nStderr: %s", filename, err, stderr.String())
	}
	return out.Bytes(), nil
}

func nodeToString(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return fmt.Sprintf("<error_converting_node: %v>", err)
	}
	return buf.String()
}

type assertionVisitor struct {
	fset           *token.FileSet
	assertions     []AssertionData
	varAssignments map[string]ast.Expr // Stores assignments like tmp1 := r.Ltr.AsInt()
}

func (v *assertionVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	// Look for assignments like: tmpX, err := r.Some.Expression() or tmpX := len(r.Array)
	if assignStmt, ok := node.(*ast.AssignStmt); ok {
		if len(assignStmt.Lhs) > 0 && len(assignStmt.Rhs) > 0 {
			// We are interested in simple assignments to an identifier,
			// potentially ignoring 'err' if it's a multi-value assignment.
			// Example: tmp1, err := r.Ltr.AsInt()  OR  tmp1 := r.Ltr.AsInt()
			if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok {
				// Store the RHS expression associated with this identifier
				// If it's `tmp, err := ...`, assignStmt.Rhs[0] is the expression.
				// If it's `tmp := ...`, assignStmt.Rhs[0] is also the expression.
				v.varAssignments[ident.Name] = assignStmt.Rhs[0]
				log.Printf("AST Visitor: Stored assignment for %s = %s", ident.Name, nodeToString(v.fset, assignStmt.Rhs[0]))
			}
		}
		return v // Continue traversal
	}

	callExpr, ok := node.(*ast.CallExpr)
	if !ok {
		return v
	}
	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return v
	}
	if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "assert" && selExpr.Sel.Name == "EqualValues" {
		if len(callExpr.Args) >= 3 {
			expectedStr := nodeToString(v.fset, callExpr.Args[1])
			actualKscExprStr := nodeToString(v.fset, callExpr.Args[2])
			// Resolve if actualArgNode is an identifier (tmp variable)
			actualNodeToParse := callExpr.Args[2]
			if ident, ok := actualNodeToParse.(*ast.Ident); ok {
				if resolvedExpr, found := v.varAssignments[ident.Name]; found {
					actualNodeToParse = resolvedExpr // Parse the expression assigned to the variable
					log.Printf("AST Visitor: Resolved %s to %s for assertion", ident.Name, nodeToString(v.fset, resolvedExpr))
				}
			}
			basePath, operation, opArg := v.parseKscActualAstNode(actualNodeToParse)

			if len(basePath) > 0 || operation != "" {
				v.assertions = append(v.assertions, AssertionData{
					ExpectedValueRaw:  expectedStr,
					BaseObjectPath:    basePath,
					Operation:         operation,
					OperationArg:      opArg,
					OriginalKscGoExpr: actualKscExprStr,
				})
			} else {
				log.Printf("Warning: Could not parse 'actual' expression AST for: %s in EqualValues", actualKscExprStr)
			}
		}
	}
	return v
}

func (v *assertionVisitor) parseKscActualAstNode(node ast.Node) (basePath []string, operation string, opArg string) {
	switch n := node.(type) {
	case *ast.SelectorExpr: // r.Field, r.Struct.Field, r.Ltr.AsInt (if AsInt is a field/method)
		var parts []string
		currExpr := ast.Expr(n)
		for {
			sel, ok := currExpr.(*ast.SelectorExpr)
			if !ok {
				if ident, okId := currExpr.(*ast.Ident); okId && ident.Name == "r" {
					break
				}
				return nil, "", "" // Should end with 'r'
			}
			parts = append(parts, toPascalCase(strings.TrimSuffix(sel.Sel.Name, "()"))) // PascalCase and remove () if it's a method treated as field
			currExpr = sel.X
		}
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 { // Reverse
			parts[i], parts[j] = parts[j], parts[i]
		}
		return parts, "", ""
	case *ast.IndexExpr: // r.Array[index]
		basePath, _, _ = v.parseKscActualAstNode(n.X)
		opArg = nodeToString(v.fset, n.Index)
		return basePath, "INDEX", opArg
	case *ast.CallExpr: // len(r.Array), r.InstanceGetter()
		if funIdent, ok := n.Fun.(*ast.Ident); ok && funIdent.Name == "len" && len(n.Args) == 1 {
			basePath, _, _ = v.parseKscActualAstNode(n.Args[0])
			return basePath, "LEN", ""
		}
		if selExpr, ok := n.Fun.(*ast.SelectorExpr); ok { // r.Ltr.AsInt()
			// Treat as path: ["Ltr", "AsInt"]
			basePath, _, _ = v.parseKscActualAstNode(selExpr)
			return basePath, "GETTER", "" // GETTER implies the path already includes the "method" name
		}
	case *ast.Ident: // Handle direct identifiers if they weren't resolved from varAssignments (e.g. r itself, or a global)
		// This case is tricky. If it's 'r', it's the root. If it's another ident,
		// it might be a package-level var or something not directly from 'r'.
		// For now, if it's 'r', it's an empty path (root). Otherwise, it's unhandled for pathing.
		if n.Name == "r" {
			return []string{}, "", "" // Represents the root object itself
		}
	}
	log.Printf("Warning: Unhandled AST node type for KSC 'actual' expression: %T -> %s", node, nodeToString(v.fset, node))
	return nil, "", ""
}

func extractAssertionsFromKscTest(filePath string) []AssertionData {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		log.Printf("Warning: Could not read/parse KSC Go test file %s: %v", filePath, err)
		return nil
	}
	visitor := &assertionVisitor{
		fset:           fset,
		assertions:     make([]AssertionData, 0),
		varAssignments: make(map[string]ast.Expr),
	}
	ast.Walk(visitor, node)
	log.Printf("Found %d assertions in %s", len(visitor.assertions), filePath)
	return visitor.assertions
}
