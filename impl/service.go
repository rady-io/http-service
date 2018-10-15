package impl

import (
	"fmt"
	. "github.com/Hexilee/impler/log"
	. "github.com/dave/jennifer/jen"
	"go/ast"
	"go/token"
	"go/types"
	"net/http"
	"strings"
	"time"
)

const (
	TokenType = "type"
	ZeroStr   = ""
	LF        = "\n"
)

func NewService(name string, service *types.Interface) *Service {
	return &Service{
		methods: make(map[token.Pos]*Method),
		name:    name,
		service: service,
		ServiceMeta: &ServiceMeta{
			header:  make(http.Header),
			cookies: make([]*http.Cookie, 0),
		},
	}
}

type (
	Service struct {
		methods     map[token.Pos]*Method
		name        string
		commentText string
		service     *types.Interface
		*ServiceMeta
	}

	ServiceMeta struct {
		baseUrl                      string
		header                       http.Header
		cookies                      []*http.Cookie
		self, pkg, implName, newFunc string
	}
)

func (srv *Service) resolveCode(file *File) (err error) {
	file.HeaderComment(fmt.Sprintf(`Implement of %s.%s
This file is generated by github.com/Hexilee/impler at %s
DON'T EDIT IT!
`, srv.pkg, srv.name, time.Now()))

	file.Func().Id(srv.newFunc).Params().Qual(srv.pkg, srv.name).BlockFunc(func(group *Group) {
		group.Id(srv.self).Op(":=").Op("&").Id(srv.implName).Values(Dict{
			Id(FieldBaseUrl): Lit(srv.baseUrl),
			Id(FieldHeader):  Make(Qual(HttpPkg, "Header")),
			Id(FieldCookies): Make(Index().Op("*").Qual(HttpPkg, "Cookie"), Lit(0)),
		})
		for key, values := range srv.header {
			for _, value := range values {
				group.Id(srv.self).Dot(FieldHeader).Dot("Add").Call(Lit(key), Lit(value))
			}
		}

		for _, cookie := range srv.cookies {
			group.Id(srv.self).Dot(FieldCookies).Op("=").Append(
				Id(srv.self).Dot(FieldCookies),
				Op("&").Qual(HttpPkg, "Cookie").Values(Dict{
					Id("Name"):  Lit(cookie.Name),
					Id("Value"): Lit(cookie.Value),
				}),
			)
		}
		group.Return(Id(srv.self))

	})

	file.Type().Id(srv.implName).Struct(
		Id(FieldBaseUrl).String(),
		Id(FieldHeader).Qual(HttpPkg, "Header"),
		Id(FieldCookies).Index().Op("*").Qual(HttpPkg, "Cookie"),
	)

	for _, method := range srv.methods {
		Log.Infof("Implement method: %s", method.String())
		err = method.resolveMetadata()
		if err != nil {
			break
		}
		method.resolveCode(file)
	}
	return
}

func (srv *Service) InitComments(cmap ast.CommentMap) *Service {
	for i := 0; i < srv.service.NumExplicitMethods(); i++ {
		rawMethod := srv.service.ExplicitMethod(i)
		srv.SetMethod(rawMethod)
	}
	for node := range cmap {
		switch tok := node.(type) {
		case *ast.GenDecl:
			if !srv.Complete() {
				srv.TrySetNode(tok)
			}
		case *ast.Field:
			srv.TryAddField(tok)
		}
	}
	return srv
}

func (srv *Service) SetMethod(rawMethod *types.Func) {
	srv.methods[rawMethod.Pos()] = NewMethod(srv, rawMethod)
}

func (srv *Service) TrySetNode(node *ast.GenDecl) {
	success := true
	if node.Tok.String() != TokenType {
		success = false
	}

	if success {
		for i := 0; i < srv.service.NumExplicitMethods(); i++ {
			method := srv.service.ExplicitMethod(i)
			if method.Pos() < node.Pos() || method.Pos() > node.End() {
				success = false
				break
			}
		}
	}

	if success {
		srv.commentText = strings.Trim(node.Doc.Text(), LF)
	}
	return
}

func (srv *Service) TryAddField(node *ast.Field) {
	if method, ok := srv.methods[node.Pos()]; ok {
		if len(node.Names) == 1 && method.Name() == node.Names[0].String() {
			method.commentText = strings.Trim(node.Doc.Text(), LF)
		}
	}
}

func (srv *Service) Complete() bool {
	return srv.commentText != ZeroStr
}

func (srv Service) String() string {
	str := new(strings.Builder)
	fmt.Fprintf(str, "/*\n%s\n*/\n", srv.commentText)
	fmt.Fprintf(str, "type %s interface {\n", srv.name)
	for _, method := range srv.methods {
		fmt.Fprintf(str, "\t/*\n%s\n\t*/\n", method.commentText)
		fmt.Fprintf(str, "\t%s(", method.Name())
		params := method.signature.Params()
		results := method.signature.Results()
		for i := 0; i < params.Len(); i++ {
			param := params.At(i)
			fmt.Fprintf(str, "%s %s", param.Name(), param.Type())
			if i != params.Len()-1 {
				fmt.Fprint(str, ", ")
			}
		}
		fmt.Fprint(str, ") (")
		for i := 0; i < results.Len(); i++ {
			result := results.At(i)
			fmt.Fprintf(str, "%s", result.Type())
			if i != results.Len()-1 {
				fmt.Fprint(str, ", ")
			}
		}
		fmt.Fprint(str, ")\n\n")
	}
	fmt.Fprintln(str, "}")
	return str.String()
}

func (srv *Service) resolveMetadata() (err error) {
	err = NewProcessor(srv.commentText).Scan(func(ann, key, value string) (err error) {
		switch ann {
		case BaseAnn:
			if srv.baseUrl != ZeroStr {
				err = DuplicatedAnnotationError(BaseAnn)
			}
			srv.setBaseUrl(value)
		case HeaderAnn:
			srv.addHeader(key, value)
		case CookieAnn:
			srv.addCookie(key, value)
		}
		return
	})
	return
}

func (srv *Service) setBaseUrl(value string) {
	Log.Debugf("Set BaseURL: %s", value)
	srv.baseUrl = value
}

func (srv *Service) addHeader(key, value string) {
	Log.Debugf("Add Header: %s(%s)", key, value)
	srv.header.Add(key, value)
}

func (srv *Service) addCookie(key, value string) {
	Log.Debugf("Add Cookie: %s(%s)", key, value)
	srv.cookies = append(srv.cookies, &http.Cookie{Name: key, Value: value})
}
