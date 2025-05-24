package main

// import (
// 	"bytes"
// 	"flag"
// 	"fmt"
// 	"go/ast"
// 	"go/format"
// 	"go/parser"
// 	"go/token"
// 	"log"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"runtime"
// 	"strconv"
// 	"strings"
// 	"text/template"
// 	"unicode"
// )

// var (
// 	projectRoot      = flag.String("project_root", "", "Project root directory (auto-detected if not provided)")
// 	ksyDir           = flag.String("ksy_dir", "test/formats", "KSY files directory (relative to project root)")
// 	binDir           = flag.String("bin_dir", "test/src", "Binary test files directory (relative to project root)")
// 	kscGoTestDir     = flag.String("ksc_go_test_dir", "test/go", "KSC Go test files directory (relative to project root)")
// 	kscGenDir        = flag.String("ksc_gen_dir", "testdata/formats_kaitai_go_gen", "KSC generated Go files directory (relative to project root)")
// 	testOutputDir    = flag.String("test_output_dir", "testdata/kaitaistruct/formats_test", "Test output directory (relative to project root)")
// 	testTemplateFile = flag.String("test_template_file", "scripts/test_template_improved.tmpl", "Template file path (relative to project root)")
// )

// type TemplateData struct {
// 	FormatName         string
// 	StructName         string
// 	KsyFileName        string
// 	FormatPackageAlias string
// 	TestCases          []TestCaseData
// 	Assertions         []AssertionData
// }

// type TestCaseData struct {
// 	Name           string
// 	BinFileRelPath string
// }

// type AssertionData struct {
// 	ExpectedValueRaw  string
// 	BaseObjectPath    []string
// 	Operation         string // "", "INDEX", "LEN", "METHOD"
// 	OperationArg      string
// 	MethodName        string // For METHOD operations (e.g., "AsInt", "AsStr")
// 	OriginalKscGoExpr string
// }

// func getProjectRoot() string {
// 	_, b, _, ok := runtime.Caller(0)
// 	if !ok {
// 		log.Fatal("Failed to get caller information to determine script directory")
// 	}
// 	scriptDir := filepath.Dir(b)
// 	return filepath.Clean(filepath.Join(scriptDir, ".."))
// }

// func main() {
// 	flag.Parse()

// 	var rootDir string
// 	if *projectRoot != "" {
// 		rootDir = *projectRoot
// 	} else {
// 		rootDir = getProjectRoot()
// 	}

// 	absKsyDir := filepath.Join(rootDir, *ksyDir)
// 	absBinDir := filepath.Join(rootDir, *binDir)
// 	absKscGoTestDir := filepath.Join(rootDir, *kscGoTestDir)
// 	absKscGenDir := filepath.Join(rootDir, *kscGenDir)
// 	absTestOutputDir := filepath.Join(rootDir, *testOutputDir)
// 	absTestTemplateFile := filepath.Join(rootDir, *testTemplateFile)

// 	log.Printf("=== Kaitai Test Generator Configuration ===")
// 	log.Printf("Project Root: %s", rootDir)
// 	log.Printf("KSY Source Directory: %s", absKsyDir)
// 	log.Printf("BIN Source Directory: %s", absBinDir)
// 	log.Printf("==========================================")

// 	if _, err := os.Stat(absTestTemplateFile); err != nil {
// 		log.Fatalf("Template file not found: %s", absTestTemplateFile)
// 	}

// 	for _, dir := range []string{absKscGenDir, absTestOutputDir} {
// 		if err := os.MkdirAll(dir, 0755); err != nil {
// 			log.Fatalf("Failed to create directory '%s': %v", dir, err)
// 		}
// 	}

// 	tmpl, err := template.New(filepath.Base(absTestTemplateFile)).
// 		Funcs(template.FuncMap{
// 			"toLower": strings.ToLower,
// 			"len": func(slice []AssertionData) int {
// 				return len(slice)
// 			},
// 			"base": filepath.Base,
// 		}).
// 		ParseFiles(absTestTemplateFile)
// 	if err != nil {
// 		log.Fatalf("Failed to parse template '%s': %v", absTestTemplateFile, err)
// 	}

// 	ksyFiles, err := filepath.Glob(filepath.Join(absKsyDir, "*.ksy"))
// 	if err != nil {
// 		log.Fatalf("Failed to glob KSY files in '%s': %v", absKsyDir, err)
// 	}

// 	if len(ksyFiles) == 0 {
// 		log.Printf("Warning: No .ksy files found in %s", absKsyDir)
// 		return
// 	}

// 	log.Printf("Found %d KSY files to process", len(ksyFiles))

// 	successCount := 0
// 	skipCount := 0

// 	for i, ksyFilePath := range ksyFiles {
// 		ksyFileName := filepath.Base(ksyFilePath)
// 		formatName := strings.TrimSuffix(ksyFileName, ".ksy")
// 		goPackageName := strings.ReplaceAll(strings.ReplaceAll(formatName, "-", "_"), ".", "_")

// 		log.Printf("[%d/%d] Processing format: %s", i+1, len(ksyFiles), formatName)

// 		cmd := exec.Command("ksc", "-t", "go", "--outdir", absKscGenDir, "--go-package", goPackageName, "--import-path", absKsyDir, ksyFilePath)
// 		var kscErr bytes.Buffer
// 		cmd.Stderr = &kscErr
// 		if err := cmd.Run(); err != nil {
// 			log.Printf("  ❌ KSC failed for %s: %v", formatName, err)
// 			skipCount++
// 			continue
// 		}

// 		// Enhanced binary file discovery
// 		testCases := findBinaryTestFiles(absBinDir, formatName)

// 		kscGoTestFilePath := filepath.Join(absKscGoTestDir, goPackageName+"_test.go")
// 		assertions := extractAssertionsFromKscTest(kscGoTestFilePath)

// 		if len(testCases) == 0 && len(assertions) == 0 {
// 			log.Printf("  ⚠️  No test data found for %s", formatName)
// 			skipCount++
// 			continue
// 		}

// 		// Augment test cases with those found by analyzing KSC Go test files
// 		testCases = augmentTestCasesFromKscGoTest(testCases, absBinDir, kscGoTestFilePath)

// 		log.Printf("  📊 Generating test: %d binary files, %d assertions", len(testCases), len(assertions))

// 		data := TemplateData{
// 			FormatName:         formatName,
// 			StructName:         toPascalCase(goPackageName),
// 			KsyFileName:        ksyFileName,
// 			FormatPackageAlias: goPackageName + "_kaitai",
// 			TestCases:          testCases,
// 			Assertions:         assertions,
// 		}

// 		outputTestFilePath := filepath.Join(absTestOutputDir, goPackageName+"_gen_test.go")
// 		if err := generateTestFile(tmpl, data, outputTestFilePath); err != nil {
// 			log.Printf("  ❌ Failed to generate test file: %v", err)
// 			skipCount++
// 			continue
// 		}

// 		log.Printf("  ✅ Generated test file")
// 		successCount++
// 	}

// 	log.Printf("\n=== Summary ===")
// 	log.Printf("Success: %d", successCount)
// 	log.Printf("Skipped: %d", skipCount)
// }

// // findBinaryTestFiles discovers binary test files based on common naming patterns.
// func findBinaryTestFiles(binDir, formatName string) []TestCaseData {
// 	var testCases []TestCaseData
// 	seen := make(map[string]bool)

// 	// Patterns to check
// 	patterns := []string{
// 		formatName + ".bin",
// 		formatName + "_*.bin",
// 		strings.ReplaceAll(formatName, "_", "-") + ".bin",
// 		strings.ReplaceAll(formatName, "-", "_") + ".bin",
// 		// Case-insensitive match for the format name itself
// 		strings.ToLower(formatName) + ".bin",
// 		strings.ToUpper(formatName) + ".bin",
// 	}

// 	for _, pattern := range patterns {
// 		matches, _ := filepath.Glob(filepath.Join(binDir, pattern))
// 		for _, match := range matches {
// 			baseName := filepath.Base(match)
// 			if !seen[baseName] {
// 				testName := strings.TrimSuffix(baseName, ".bin")
// 				testCases = append(testCases, TestCaseData{
// 					Name:           testName,
// 					BinFileRelPath: baseName,
// 				})
// 				seen[baseName] = true
// 			}
// 		}
// 	}
// 	if len(testCases) > 0 {
// 		log.Printf("  Found %d binary test files using glob patterns", len(testCases))
// 	}
// 	return testCases
// }

// // augmentTestCasesFromKscGoTest adds test cases found by parsing os.Open calls in KSC's Go test file.
// func augmentTestCasesFromKscGoTest(existingTestCases []TestCaseData, binDir, kscGoTestFilePath string) []TestCaseData {
// 	seen := make(map[string]bool)
// 	for _, tc := range existingTestCases {
// 		seen[tc.BinFileRelPath] = true
// 	}

// 	extractedBinFiles := extractBinFilenamesFromGoTest(kscGoTestFilePath)
// 	addedCount := 0
// 	for _, binFile := range extractedBinFiles {
// 		if !seen[binFile] {
// 			// Check if this file actually exists in our binDir
// 			if _, err := os.Stat(filepath.Join(binDir, binFile)); err == nil {
// 				testName := strings.TrimSuffix(binFile, ".bin")
// 				existingTestCases = append(existingTestCases, TestCaseData{
// 					Name:           testName,
// 					BinFileRelPath: binFile,
// 				})
// 				seen[binFile] = true
// 				addedCount++
// 			} else {
// 				log.Printf("  🔍 Bin file '%s' mentioned in KSC test '%s' but not found in '%s'", binFile, filepath.Base(kscGoTestFilePath), binDir)
// 			}
// 		}
// 	}
// 	if addedCount > 0 {
// 		log.Printf("  Added %d binary test files by analyzing KSC Go test: %s", addedCount, filepath.Base(kscGoTestFilePath))
// 	}
// 	return existingTestCases
// }

// // Helper struct for visiting AST nodes to find os.Open calls
// type osOpenVisitor struct {
// 	fset               *token.FileSet
// 	discoveredBinFiles map[string]bool // Use a map for uniqueness
// }

// func (v *osOpenVisitor) Visit(node ast.Node) ast.Visitor {
// 	if node == nil {
// 		return nil
// 	}

// 	callExpr, ok := node.(*ast.CallExpr)
// 	if !ok {
// 		return v
// 	}

// 	// Check if it's os.Open
// 	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
// 	if !ok {
// 		return v
// 	}

// 	if pkgIdent, ok := selExpr.X.(*ast.Ident); ok && pkgIdent.Name == "os" && selExpr.Sel.Name == "Open" {
// 		if len(callExpr.Args) > 0 {
// 			if pathLit, ok := callExpr.Args[0].(*ast.BasicLit); ok && pathLit.Kind == token.STRING {
// 				pathArg, err := strconv.Unquote(pathLit.Value)
// 				if err == nil {
// 					baseName := filepath.Base(pathArg)
// 					if strings.HasSuffix(strings.ToLower(baseName), ".bin") { // Case-insensitive check for .bin
// 						v.discoveredBinFiles[baseName] = true
// 					}
// 				}
// 			}
// 		}
// 	}
// 	return v
// }

// // extractBinFilenamesFromGoTest parses a Go test file and extracts .bin filenames
// // from os.Open(".../filename.bin") calls.
// func extractBinFilenamesFromGoTest(filePath string) []string {
// 	if _, err := os.Stat(filePath); os.IsNotExist(err) {
// 		// KSC Go test file might not exist for all formats, which is fine.
// 		return nil
// 	}

// 	fset := token.NewFileSet()
// 	node, err := parser.ParseFile(fset, filePath, nil, 0)
// 	if err != nil {
// 		log.Printf("Warning: Could not parse KSC Go test file %s for os.Open calls: %v", filePath, err)
// 		return nil
// 	}

// 	visitor := &osOpenVisitor{
// 		fset:               fset,
// 		discoveredBinFiles: make(map[string]bool),
// 	}
// 	ast.Walk(visitor, node)

// 	var filenames []string
// 	for fname := range visitor.discoveredBinFiles {
// 		filenames = append(filenames, fname)
// 	}
// 	return filenames
// }

// func generateTestFile(tmpl *template.Template, data TemplateData, outputPath string) error {
// 	outFile, err := os.Create(outputPath)
// 	if err != nil {
// 		return fmt.Errorf("failed to create test file %s: %w", outputPath, err)
// 	}
// 	defer outFile.Close()

// 	var renderedTest bytes.Buffer
// 	if err := tmpl.Execute(&renderedTest, data); err != nil {
// 		return fmt.Errorf("failed to execute template: %w", err)
// 	}

// 	formattedBytes, err := formatSource(outputPath, renderedTest.Bytes())
// 	if err != nil {
// 		log.Printf("Warning: goimports failed for %s: %v", outputPath, err)
// 		formattedBytes = renderedTest.Bytes()
// 	}

// 	if _, err := outFile.Write(formattedBytes); err != nil {
// 		return fmt.Errorf("failed to write to file: %w", err)
// 	}

// 	return nil
// }

// func toPascalCase(s string) string {
// 	var result strings.Builder
// 	capitalizeNext := true
// 	for _, r := range s {
// 		if r == '_' || r == '-' {
// 			capitalizeNext = true
// 			continue
// 		}
// 		if capitalizeNext {
// 			result.WriteRune(unicode.ToUpper(r))
// 			capitalizeNext = false
// 		} else {
// 			result.WriteRune(r)
// 		}
// 	}
// 	return result.String()
// }

// func formatSource(filename string, src []byte) ([]byte, error) {
// 	cmd := exec.Command("goimports", "-srcdir", filepath.Dir(filename))
// 	cmd.Stdin = bytes.NewReader(src)
// 	var out bytes.Buffer
// 	cmd.Stdout = &out
// 	var stderr bytes.Buffer
// 	cmd.Stderr = &stderr
// 	if err := cmd.Run(); err != nil {
// 		return src, fmt.Errorf("goimports failed: %w\nStderr: %s", err, stderr.String())
// 	}
// 	return out.Bytes(), nil
// }

// func nodeToString(fset *token.FileSet, node ast.Node) string {
// 	var buf bytes.Buffer
// 	if err := format.Node(&buf, fset, node); err != nil {
// 		return fmt.Sprintf("<error: %v>", err)
// 	}
// 	return buf.String()
// }

// type assertionVisitor struct {
// 	fset           *token.FileSet
// 	assertions     []AssertionData
// 	varAssignments map[string]ast.Expr
// }

// func (v *assertionVisitor) Visit(node ast.Node) ast.Visitor {
// 	if node == nil {
// 		return nil
// 	}

// 	if assignStmt, ok := node.(*ast.AssignStmt); ok {
// 		if len(assignStmt.Lhs) > 0 && len(assignStmt.Rhs) > 0 {
// 			if ident, ok := assignStmt.Lhs[0].(*ast.Ident); ok {
// 				v.varAssignments[ident.Name] = assignStmt.Rhs[0]
// 			}
// 		}
// 		return v
// 	}

// 	callExpr, ok := node.(*ast.CallExpr)
// 	if !ok {
// 		return v
// 	}
// 	selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
// 	if !ok {
// 		return v
// 	}
// 	if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "assert" && selExpr.Sel.Name == "EqualValues" {
// 		if len(callExpr.Args) >= 3 {
// 			expectedStr := nodeToString(v.fset, callExpr.Args[1])
// 			actualKscExprStr := nodeToString(v.fset, callExpr.Args[2])

// 			actualNodeToParse := callExpr.Args[2]
// 			if ident, ok := actualNodeToParse.(*ast.Ident); ok {
// 				if resolvedExpr, found := v.varAssignments[ident.Name]; found {
// 					actualNodeToParse = resolvedExpr
// 				}
// 			}

// 			basePath, operation, opArg, methodName := v.parseKscActualAstNode(actualNodeToParse)

// 			if len(basePath) > 0 || operation != "" {
// 				v.assertions = append(v.assertions, AssertionData{
// 					ExpectedValueRaw:  expectedStr,
// 					BaseObjectPath:    basePath,
// 					Operation:         operation,
// 					OperationArg:      opArg,
// 					MethodName:        methodName,
// 					OriginalKscGoExpr: actualKscExprStr,
// 				})
// 			}
// 		}
// 	}
// 	return v
// }

// func (v *assertionVisitor) parseKscActualAstNode(node ast.Node) (basePath []string, operation string, opArg string, methodName string) {
// 	switch n := node.(type) {
// 	case *ast.CallExpr:
// 		// Handle len(r.Array)
// 		if funIdent, ok := n.Fun.(*ast.Ident); ok && funIdent.Name == "len" && len(n.Args) == 1 {
// 			basePath, _, _, _ = v.parseKscActualAstNode(n.Args[0])
// 			return basePath, "LEN", "", ""
// 		}

// 		// Handle method calls like r.Ltr.AsInt() or r.Header.Flags.IsCompressed()
// 		if selExpr, ok := n.Fun.(*ast.SelectorExpr); ok {
// 			methodName = selExpr.Sel.Name
// 			// Check if this is a known conversion method
// 			if isConversionMethod(methodName) {
// 				basePath, _, _, _ = v.parseKscActualAstNode(selExpr.X)
// 				return basePath, "METHOD", "", methodName
// 			}
// 			// If not a special conversion method, it might be a boolean flag or similar.
// 			// Treat it as part of the path for now.
// 			// This might need refinement if it's a method that returns a complex type.
// 			parentPath, _, _, _ := v.parseKscActualAstNode(selExpr.X)
// 			basePath = append(parentPath, toPascalCase(methodName)) // Add method name to path
// 			return basePath, "", "", ""                             // No specific operation, just path access
// 		}

// 	case *ast.SelectorExpr:
// 		var parts []string
// 		currExpr := ast.Expr(n)
// 		for {
// 			sel, ok := currExpr.(*ast.SelectorExpr)
// 			if !ok {
// 				if ident, okId := currExpr.(*ast.Ident); okId && ident.Name == "r" {
// 					break
// 				}
// 				return nil, "", "", ""
// 			}
// 			parts = append(parts, toPascalCase(sel.Sel.Name))
// 			currExpr = sel.X
// 		}
// 		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
// 			parts[i], parts[j] = parts[j], parts[i]
// 		}
// 		return parts, "", "", ""

// 	case *ast.IndexExpr:
// 		basePath, _, _, _ = v.parseKscActualAstNode(n.X)
// 		opArg = nodeToString(v.fset, n.Index)
// 		return basePath, "INDEX", opArg, ""

// 	case *ast.Ident:
// 		if n.Name == "r" {
// 			return []string{}, "", "", ""
// 		}
// 	}

// 	return nil, "", "", ""
// }

// // Known conversion methods that should be handled specially
// func isConversionMethod(name string) bool {
// 	methods := map[string]bool{
// 		"AsInt":  true,
// 		"AsStr":  true,
// 		"String": true,
// 		"Int":    true,
// 	}
// 	return methods[name]
// }

// func extractAssertionsFromKscTest(filePath string) []AssertionData {
// 	fset := token.NewFileSet()
// 	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
// 	if err != nil {
// 		log.Printf("Warning: Could not parse KSC test file %s: %v", filePath, err)
// 		return nil
// 	}

// 	visitor := &assertionVisitor{
// 		fset:           fset,
// 		assertions:     make([]AssertionData, 0),
// 		varAssignments: make(map[string]ast.Expr),
// 	}

// 	ast.Walk(visitor, node)
// 	return visitor.assertions
// }
