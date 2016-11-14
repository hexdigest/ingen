package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/imports"
)

type Visitor struct {
	*loader.PackageInfo
	FoundTypes []string
}

func (v *Visitor) Visit(n ast.Node) ast.Visitor {
	if n != nil {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if t := v.PackageInfo.Info.TypeOf(ts.Type); t != nil && isComparable(t) {
				v.FoundTypes = append(v.FoundTypes, ts.Name.Name)
			}
		}
		return v
	}
	return nil
}

func isComparable(t types.Type) bool {
	switch v := t.(type) {
	case *types.Slice, *types.Map:
		return false
	case *types.Array:
		return isComparable(v.Elem())
	case *types.Named:
		return isComparable(v.Underlying())
	case *types.Struct:
		for i := 0; i < v.NumFields(); i++ {
			if !isComparable(v.Field(i).Type()) {
				return false
			}
		}
	}
	return true
}

func main() {
	pkgPath := flag.String("p", "", "package to parse")
	flag.Parse()

	if pkgPath == nil {
		flag.PrintDefaults()
		os.Exit(1)
	}

	conf := loader.Config{}
	conf.Import(*pkgPath)
	prog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}

	pkg := prog.Package(*pkgPath)

	v := &Visitor{PackageInfo: pkg}
	for _, f := range pkg.Files {
		ast.Walk(v, f)
	}

	buf := bytes.NewBuffer([]byte{})

	template.Must(template.New("").Parse(`
		//This code was automatically generated. Please don't modify
		package {{.Pkg.Name}}

		{{ range .FoundTypes }}
			func (v {{.}}) In(list ...{{.}}) bool {
				for _, l := range list {
					if v == l {
						return true
					}
				}
				return false
			}
		{{ end }}
	`)).Execute(buf, v)

	var pkgAbsPath string
	for _, path := range filepath.SplitList(os.Getenv("GOPATH")) {
		fullPath := filepath.Join(path, "src", *pkgPath)
		pkgAbsPath, err = filepath.Abs(fullPath)
		if err != nil {
			continue
		}

		if _, err = os.Stat(fullPath); err != nil {
			continue
		}
	}

	outFile := filepath.Join(pkgAbsPath, "type_helpers.go")

	formatted, err := imports.Process(outFile, buf.Bytes(), nil)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create(outFile)
	if err != nil {
		log.Fatalf("failed to create file: %v", err)
	}

	if _, err := f.Write(formatted); err != nil {
		log.Fatalf("failed to write formatted source: %v", err)
	}

	if err := f.Close(); err != nil {
		log.Fatalf("failed to close file: %v", err)
	}
}
