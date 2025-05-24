package main

// import (
// 	"bytes"
// 	"fmt"
// 	"log"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"runtime"
// 	"strings"
// 	"text/template"
// 	"unicode"
// )

// const (
// 	// projectRootDefault    = "../" // Will be determined dynamically
// 	ksyDirRelative        = "test/formats"                   // Changed
// 	binDirRelative        = "test/src"                       // Added: for .bin files
// 	kscGenDirRelative     = "testdata/formats_kaitai_go_gen" // Stays the same, our internal gen dir
// 	testOutputDirRelative = "testdata/kaitaistruct/formats_test"
// 	testTemplateFile      = "scripts/test_template.tmpl" // Path relative to project root
// )

// type TemplateData struct {
// 	FormatName         string // e.g., "png", "bits_simple"
// 	StructName         string // e.g., "Png", "BitsSimple" (PascalCase)
// 	KsyFileName        string // e.g., "png.ksy"
// 	FormatPackageAlias string // e.g., "png_kaitai"
// 	TestCases          []TestCaseData
// }

// type TestCaseData struct {
// 	Name           string // e.g., "sample1" (from sample1.bin)
// 	BinFileRelPath string // Relative path of .bin file from ksyDir/FormatName/
// }

// func getProjectRoot() string {
// 	// Get the directory of the currently running script
// 	_, b, _, ok := runtime.Caller(0)
// 	if !ok {
// 		log.Fatal("Failed to get caller information to determine script directory")
// 	}
// 	scriptDir := filepath.Dir(b) // Directory of generate_kaitai_tests.go
// 	// Assume script is in PROJECT_ROOT/scripts/
// 	// So, project root is one level up from scriptDir
// 	return filepath.Clean(filepath.Join(scriptDir, ".."))
// }

// func main() {
// 	projectRoot := getProjectRoot()

// 	absKsyDir := filepath.Join(projectRoot, ksyDirRelative)
// 	absBinDir := filepath.Join(projectRoot, binDirRelative) // Added
// 	absKscGenDir := filepath.Join(projectRoot, kscGenDirRelative)
// 	absTestOutputDir := filepath.Join(projectRoot, testOutputDirRelative)
// 	absTestTemplateFile := filepath.Join(projectRoot, testTemplateFile)

// 	log.Printf("Project Root: %s", projectRoot)
// 	log.Printf("KSY Source Directory (for .ksy files): %s", absKsyDir)
// 	log.Printf("KSY Directory: %s", absKsyDir)
// 	log.Printf("KSC Gen Directory: %s", absKscGenDir)
// 	log.Printf("Test Output Directory: %s", absTestOutputDir)
// 	log.Printf("Test Template: %s", absTestTemplateFile)

// 	if err := os.MkdirAll(absKscGenDir, 0755); err != nil {
// 		log.Fatalf("Failed to create KSC gen dir '%s': %v", absKscGenDir, err)
// 	}
// 	if err := os.MkdirAll(absTestOutputDir, 0755); err != nil {
// 		log.Fatalf("Failed to create test output dir '%s': %v", absTestOutputDir, err)
// 	}

// 	tmpl, err := template.New(filepath.Base(absTestTemplateFile)).ParseFiles(absTestTemplateFile)
// 	if err != nil {
// 		log.Fatalf("Failed to parse template '%s': %v", absTestTemplateFile, err)
// 	}

// 	ksyFiles, err := filepath.Glob(filepath.Join(absKsyDir, "*.ksy"))
// 	if err != nil {
// 		log.Fatalf("Failed to glob KSY files in '%s': %v", absKsyDir, err)
// 	}

// 	for _, ksyFilePath := range ksyFiles {
// 		ksyFileName := filepath.Base(ksyFilePath)
// 		formatName := strings.TrimSuffix(ksyFileName, ".ksy")

// 		log.Printf("Processing format: %s (KSY: %s)", formatName, ksyFileName)

// 		// 1. Run KSC
// 		// kscFormatOutDir := filepath.Join(absKscGenDir, formatName) // We don't need to create this sub-directory manually
// 		// if err := os.MkdirAll(kscFormatOutDir, 0755); err != nil {
// 		// 	log.Printf("Failed to create KSC output dir for %s: %v. Skipping.", formatName, err)
// 		// 	continue
// 		// }
// 		goPackageName := strings.ReplaceAll(formatName, "-", "_")   // Basic sanitization
// 		goPackageName = strings.ReplaceAll(goPackageName, ".", "_") // e.g. for fixed_struct.ksy

// 		// Inside the loop in generate_kaitai_tests.go, when constructing the cmd
// 		cmd := exec.Command("ksc",
// 			"-t", "go",
// 			"--outdir", absKscGenDir, // Corrected: Use the base KSC gen directory
// 			"--go-package", goPackageName,
// 			"--import-path", absKsyDir, // Add the directory containing the KSY files as an import path
// 			ksyFilePath,
// 		)

// 		var kscErr bytes.Buffer
// 		cmd.Stderr = &kscErr
// 		if err := cmd.Run(); err != nil {
// 			log.Printf("KSC failed for %s (KSY: %s): %v. Stderr: %s. Skipping test generation.", formatName, ksyFilePath, err, kscErr.String())
// 			continue
// 		}

// 		// 2. Find sample .bin files
// 		var testCases []TestCaseData
// 		// For this structure, the .bin file is expected in absBinDir with the same base name.
// 		binFilePath := filepath.Join(absBinDir, formatName+".bin")
// 		if _, err := os.Stat(binFilePath); err == nil {
// 			testCases = append(testCases, TestCaseData{
// 				Name:           formatName,          // Use formatName as the test case name
// 				BinFileRelPath: formatName + ".bin", // Path relative to absBinDir
// 			})
// 		}

// 		if len(testCases) == 0 {
// 			log.Printf("No .bin sample files found for %s in expected locations. Test file will be generated without binary-specific test cases.", formatName)
// 			// Removed: continue
// 		}

// 		// 3. Prepare template data
// 		structName := toPascalCase(goPackageName)
// 		data := TemplateData{
// 			FormatName:         formatName,
// 			StructName:         structName,
// 			KsyFileName:        ksyFileName,
// 			FormatPackageAlias: goPackageName + "_kaitai",
// 			TestCases:          testCases,
// 		}

// 		// 4. Generate test file
// 		outputTestFilePath := filepath.Join(absTestOutputDir, goPackageName+"_gen_test.go") // Use goPackageName for file
// 		outFile, err := os.Create(outputTestFilePath)
// 		if err != nil {
// 			log.Fatalf("Failed to create test file %s: %v", outputTestFilePath, err)
// 		}

// 		var renderedTest bytes.Buffer
// 		if err := tmpl.Execute(&renderedTest, data); err != nil {
// 			outFile.Close() // Close before failing
// 			log.Fatalf("Failed to execute template for %s: %v", formatName, err)
// 		}

// 		// Use goimports to format and add missing imports
// 		formattedBytes, err := formatSource(outputTestFilePath, renderedTest.Bytes())
// 		if err != nil {
// 			log.Printf("Warning: goimports failed for %s: %v. Writing unformatted code.", outputTestFilePath, err)
// 			// Write unformatted if goimports fails
// 			if _, err := outFile.Write(renderedTest.Bytes()); err != nil {
// 				log.Printf("Failed to write unformatted code to %s: %v", outputTestFilePath, err)
// 			}
// 		} else {
// 			if _, err := outFile.Write(formattedBytes); err != nil {
// 				log.Printf("Failed to write formatted code to %s: %v", outputTestFilePath, err)
// 			}
// 		}
// 		outFile.Close()
// 		log.Printf("Generated test file: %s", outputTestFilePath)
// 	}
// 	log.Println("Test generation script finished.")
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

// // formatSource runs goimports on the given content.
// // filename is used by goimports for context, but content is read from src.
// func formatSource(filename string, src []byte) ([]byte, error) {
// 	cmd := exec.Command("goimports", "-srcdir", filepath.Dir(filename))
// 	cmd.Stdin = bytes.NewReader(src)
// 	var out bytes.Buffer
// 	cmd.Stdout = &out
// 	var stderr bytes.Buffer
// 	cmd.Stderr = &stderr

// 	if err := cmd.Run(); err != nil {
// 		return src, fmt.Errorf("goimports failed for %s: %w\nStderr: %s", filename, err, stderr.String())
// 	}
// 	return out.Bytes(), nil
// }
