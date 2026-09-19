// Command surfacecheck verifies the checked-in public declaration baseline.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	root := flag.String("root", ".", "module root")
	printSurface := flag.Bool("print", false, "print the discovered surface")
	flag.Parse()
	actual, err := discover(*root)
	if err != nil {
		fail(err)
	}
	if *printSurface {
		for _, name := range actual {
			fmt.Println(name)
		}
		return
	}
	want, err := readBaseline(filepath.Join(*root, "internal/tools/surfacecheck/surface.txt"))
	if err != nil {
		fail(err)
	}
	missing, unexpected := difference(want, actual), difference(actual, want)
	if len(missing) > 0 || len(unexpected) > 0 {
		for _, name := range missing {
			fmt.Fprintf(os.Stderr, "missing: %s\n", name)
		}
		for _, name := range unexpected {
			fmt.Fprintf(os.Stderr, "unexpected: %s\n", name)
		}
		os.Exit(1)
	}
}

func discover(root string) ([]string, error) {
	packages := []struct{ label, path string }{{"typesafe", root}, {"option", filepath.Join(root, "option")}}
	var names []string
	for _, item := range packages {
		parsed, err := parser.ParseDir(token.NewFileSet(), item.path, func(info os.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, 0)
		if err != nil {
			return nil, err
		}
		pkg := parsed[item.label]
		if pkg == nil {
			return nil, fmt.Errorf("package %s not found in %s", item.label, item.path)
		}
		for _, file := range pkg.Files {
			for _, declaration := range file.Decls {
				switch d := declaration.(type) {
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if s.Name.IsExported() {
								names = append(names, item.label+"."+s.Name.Name)
							}
						case *ast.ValueSpec:
							for _, name := range s.Names {
								if name.IsExported() {
									names = append(names, item.label+"."+name.Name)
								}
							}
						}
					}
				case *ast.FuncDecl:
					if !d.Name.IsExported() {
						continue
					}
					name := item.label + "." + d.Name.Name
					if d.Recv != nil && len(d.Recv.List) == 1 {
						receiver := receiverName(d.Recv.List[0].Type)
						if !ast.IsExported(receiver) {
							continue
						}
						name = item.label + "." + receiver + "." + d.Name.Name
					}
					names = append(names, name)
				}
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

func receiverName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverName(value.X)
	default:
		return "?"
	}
}

func readBaseline(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var names []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			names = append(names, line)
		}
	}
	sort.Strings(names)
	return names, scanner.Err()
}

func difference(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, value := range b {
		set[value] = true
	}
	var out []string
	for _, value := range a {
		if !set[value] {
			out = append(out, value)
		}
	}
	return out
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
