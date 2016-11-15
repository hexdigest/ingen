package main

import (
	"flag"
	"go/ast"
	"go/types"
	"log"
	"os"
	"path/filepath"

	"github.com/gojuno/generator"

	"golang.org/x/tools/go/loader"
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

	gen := generator.New(prog)
	gen.ImportWithAlias(*pkgPath, "")
	gen.SetPackageName(v.Pkg.Name())
	gen.SetHeader("This code was automatically generated. Please don't modify")

	for _, f := range pkg.Files {
		ast.Walk(v, f)
	}

	gen.ProcessTemplate("", `
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
	`, v)

	pkgAbsPath, err := generator.PackageAbsPath(*pkgPath)
	if err != nil {
		log.Fatal(err)
	}

	gen.WriteToFilename(filepath.Join(pkgAbsPath, "type_helpers.go"))
}
