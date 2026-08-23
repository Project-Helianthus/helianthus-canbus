package canbus

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestListenerPublicSurfaceIsReceiveOnly(t *testing.T) {
	t.Parallel()

	typeOfListener := reflect.TypeOf((*Listener)(nil)).Elem()
	got := make([]string, 0, typeOfListener.NumMethod())
	for i := 0; i < typeOfListener.NumMethod(); i++ {
		got = append(got, typeOfListener.Method(i).Name)
	}
	sort.Strings(got)
	want := []string{"Close", "Receive", "Stats"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Listener methods = %v, want exactly %v", got, want)
	}
}

func TestProductCodeHasNoOutboundOrMutationSurface(t *testing.T) {
	t.Parallel()

	if err := inspectProductSource("."); err != nil {
		t.Fatal(err)
	}
}

func TestTestsDoNotOpenAPlatformEndpoint(t *testing.T) {
	t.Parallel()

	for _, path := range goFiles(t, ".", true) {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			switch identifier.Name {
			case "ListenSocketCAN", "platformOpenSocketCAN":
				t.Errorf("%s invokes live endpoint entry point %s", path, identifier.Name)
			}
			return true
		})
	}
}

func TestRepositoryHasNoDomainSpecificAssumptions(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"Gr" + "ee",
		"Grow" + "att",
		"V" + "RF",
		"H" + "VAC",
		"M" + "94",
		"M" + "115",
		"20" + " kbit/s",
		"20" + "kbit/s",
		"CAN" + "+",
	}
	legacyTerms := []string{"m" + "aster", "sl" + "ave"}
	blocked = append(blocked, legacyTerms...)

	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, term := range blocked {
			if strings.Contains(strings.ToLower(string(contents)), strings.ToLower(term)) {
				return fmt.Errorf("%s contains forbidden term %q", path, term)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPlatformFilesHaveExplicitBuildTags(t *testing.T) {
	t.Parallel()

	assertFileContains(t, "socketcan_linux.go", "//go:build linux")
	assertFileContains(t, "socketcan_unsupported.go", "//go:build !linux")
}

func inspectProductSource(root string) error {
	for _, path := range goFilesFromRoot(root, false) {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, "\"")
			if strings.Contains(importPath, "netlink") || strings.Contains(importPath, "rtnetlink") {
				return fmt.Errorf("%s imports interface-mutation package %q", path, importPath)
			}
		}

		var inspectionErr error
		ast.Inspect(file, func(node ast.Node) bool {
			if inspectionErr != nil {
				return false
			}
			switch typed := node.(type) {
			case *ast.FuncDecl:
				name := strings.ToLower(typed.Name.Name)
				if strings.Contains(name, "send") || strings.Contains(name, "transmit") || strings.Contains(name, "writeframe") {
					inspectionErr = fmt.Errorf("%s declares forbidden operation %s", path, typed.Name.Name)
					return false
				}
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch selector.Sel.Name {
				case "Write", "WriteTo", "WriteMsg", "Send", "Sendto", "Sendmsg", "Transmit":
					inspectionErr = fmt.Errorf("%s calls forbidden operation %s", path, selector.Sel.Name)
					return false
				}
			case *ast.TypeSpec:
				structure, ok := typed.Type.(*ast.StructType)
				if !ok || !ast.IsExported(typed.Name.Name) {
					return true
				}
				for _, field := range structure.Fields.List {
					for _, name := range field.Names {
						switch name.Name {
						case "FD", "Socket", "Backend", "Connection":
							inspectionErr = fmt.Errorf("%s exposes transport handle field %s.%s", path, typed.Name.Name, name.Name)
							return false
						}
					}
				}
			}
			return true
		})
		if inspectionErr != nil {
			return inspectionErr
		}
	}
	return nil
}

func goFiles(t *testing.T, root string, testsOnly bool) []string {
	t.Helper()
	files := goFilesFromRoot(root, testsOnly)
	if len(files) == 0 {
		t.Fatalf("no Go files found below %s", root)
	}
	return files
}

func goFilesFromRoot(root string, testsOnly bool) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		isTest := strings.HasSuffix(path, "_test.go")
		if testsOnly != isTest {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files
}

func assertFileContains(t *testing.T, path, expected string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(contents), expected) {
		t.Fatalf("%s does not contain %q", path, expected)
	}
}
