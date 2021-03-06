package impl

import (
	. "github.com/rady-io/http-service/log"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
)

const (
	Src = `
package types

import (
	"io"
	"net/http"
)

var (
	IOReader 	io.Reader
	Err      	error
	StatusCode	int
	Request		*http.Request
	Response	*http.Response
)
`
)

const (
	TypeIOReader   = "IOReader"
	TypeErr        = "Err"
	TypeStatusCode = "StatusCode"
	TypeRequest    = "Request"
	TypeResponse   = "Response"
)

func GetType(name string) types.Type {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "types.go", Src, 0)
	if err != nil {
		Log.Fatal(err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("impler/types", fset, []*ast.File{file}, nil)
	if err != nil {
		Log.Fatal(err)
	}
	return pkg.Scope().Lookup(name).Type()
}
