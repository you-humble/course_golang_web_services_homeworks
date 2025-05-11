package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strings"
)

const (
	markForCodegen string = "apigen:api"
	tagPrefix      string = "`apivalidator" // (`) - required symbol
)

// {"url": "/user/create", "auth": true, "method": "POST"}
type Endpoint struct {
	Url        string `json:"url,omitempty"`
	Auth       bool   `json:"auth,omitempty"`
	Method     string `json:"method,omitempty"`
	SeviceName string
	Params     []Param
}

func NewEndpoint(dataJSON string) (Endpoint, error) {
	e := Endpoint{}
	if err := json.Unmarshal([]byte(dataJSON), &e); err != nil {
		return Endpoint{}, err
	}

	return e, nil
}

func (e Endpoint) handlerName() string {
	return "handler" + e.SeviceName
}

type Param struct {
	Name   string
	Fields []*Field
}

// код писать тут
func main() {
	// the working file set for the parser (positions, strings and symbols)
	fset := token.NewFileSet()

	// parse comments from the first[1] argument of a command './codegen[0] api.go[1] api_handlers.go[2]'
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("Parsing file error: %+v", err)
	}

	// create an out file from the second[2] argument of a command './codegen[0] api.go[1] api_handlers.go[2]'
	out, err := os.Create(os.Args[2])
	if err != nil {
		log.Fatalf("Creating out file error: %+v", err)
	}

	printImports(out, node.Name.Name)
	printResultStruct(out)

	// parsing param structs and methods
	endpoints := make(map[string][]Endpoint)
	endpointParams := make(map[string]Param)
	for _, f := range node.Decls { // AST tree
		parseStruct(f, endpointParams)
		if err := parseEndpoints(f, endpoints, endpointParams); err != nil {
			log.Printf("Parse endpoints error: %+v\n", err)
		}
	}

	for structName, values := range endpoints {
		renderServeHTTP(out, structName, values)
		for _, endpoint := range values {
			renderHandler(out, structName, endpoint)
			renderValidate(out, endpoint)
		}
	}

	printCheckMethodMiddleware(out)
	printAuthMiddleware(out)
	printWriteResponse(out)
	printWriteJSON(out)
	printBodyToMap(out)
}

// parseStruct finds a struct and appends it to the map.
func parseStruct(declaration ast.Decl, endpointParams map[string]Param) {
	// Verify whether the declaration is a general declaration
	g, ok := declaration.(*ast.GenDecl)
	if !ok {
		return
	}

	for _, spec := range g.Specs { // range of specifications (*ImportSpec, *ValueSpec, and *TypeSpec)
		typeSpec, ok := spec.(*ast.TypeSpec) // A TypeSpec node represents a type declaration
		if !ok {
			continue
		}

		curStruct, ok := typeSpec.Type.(*ast.StructType) // A StructType node represents a struct type
		if !ok {
			continue
		}

		structName := typeSpec.Name.Name // struct name
		var paramFields []*Field         // struct fields
		for _, field := range curStruct.Fields.List {
			var fieldType, fieldTag string   // field type and tag
			v, ok := field.Type.(*ast.Ident) // An Ident node represents an identifier
			if ok {
				fieldType = v.Name // field type
			}

			if field.Tag != nil {
				tagReflect := reflect.StructTag(field.Tag.Value) // the tag string in a struct field
				fieldTag = tagReflect.Get(tagPrefix)             // field tag
			}

			structField := NewField(field.Names[0].Name, fieldType, fieldTag)
			paramFields = append(paramFields, structField)
		}
		param := Param{Name: structName, Fields: paramFields}
		endpointParams[structName] = param
	}
}

// parseEndpoints finds a struct with tagged methods and appends it to the map.
func parseEndpoints(
	declaration ast.Decl,
	endpoints map[string][]Endpoint,
	endpointParams map[string]Param,
) error {
	// Verify whether the declaration is a function declaration
	function, ok := declaration.(*ast.FuncDecl) // A FuncDecl node represents a function declaration
	if !ok || !strings.Contains(function.Doc.Text(), markForCodegen) {
		return nil
	}

	var params []Param
	for _, param := range function.Type.Params.List[1:] { // (incoming) parameters list
		argName := param.Type.(*ast.Ident).Name // An Ident node represents an identifier
		params = append(params, endpointParams[argName])
	}

	var structName string
	for _, m := range function.Recv.List { // receiver (methods); or nil (functions)
		switch mType := m.Type.(type) { // field/method/parameter type; or nil
		case *ast.StarExpr: // A StarExpr node represents an expression of the form "*" Expression.
			structName = "*" + mType.X.(*ast.Ident).Name // X - operand. An Ident node represents an identifier
		case *ast.Ident:
			structName = "*" + mType.Name
		}
	}

	var endpoint Endpoint
	var err error
	for _, comment := range function.Doc.List { // associated documentation
		dataJSON := strings.SplitN(comment.Text, " ", 3)
		endpoint, err = NewEndpoint(dataJSON[2])
		if err != nil {
			return err
		}
	}

	endpoint.SeviceName = function.Name.Name
	endpoint.Params = params

	endpoints[structName] = append(endpoints[structName], endpoint)
	return nil
}
