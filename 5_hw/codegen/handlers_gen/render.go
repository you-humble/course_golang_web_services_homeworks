package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	serverPrefix string = "srv"
)

func renderServeHTTP(out *os.File, structName string, values []Endpoint) {
	fmt.Fprintf(out,
		"func (%s %s) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n",
		serverPrefix, structName,
	)
	fmt.Fprintln(out, "\tswitch r.URL.Path {")
	for _, endpoint := range values {
		acceptedMethods := []string{}
		if endpoint.Method == http.MethodGet || endpoint.Method == "" {
			acceptedMethods = append(acceptedMethods, "http.MethodGet")
		}
		if endpoint.Method == http.MethodPost || endpoint.Method == "" {
			acceptedMethods = append(acceptedMethods, "http.MethodPost")
		}

		fmt.Fprintf(out,
			`	case %q:
		handler := %s.%s
		handler = checkMethodMiddleware(
			handler, []string{
				%s,
			})
`, endpoint.Url,
			serverPrefix,
			endpoint.handlerName(),
			strings.Join(acceptedMethods, ",\n\t\t\t\t"),
		)
		if endpoint.Auth {
			fmt.Fprintln(out, "\t\thandler = authMiddleware(handler)")
		}
		fmt.Fprintln(out, "\t\thandler(w, r)")
	}
	fmt.Fprintln(out,
		`	default:
		writeResponse(w, nil, errors.New("unknown method"), http.StatusNotFound)
	}`)
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)
}

func renderHandler(out *os.File, structName string, endpoint Endpoint) {
	fmt.Fprintf(out,
		"func (%s %s) %s(w http.ResponseWriter, r *http.Request) {\n",
		serverPrefix, structName, endpoint.handlerName())
	if endpoint.Method == "" {
		for _, p := range endpoint.Params {
			for _, f := range p.Fields {
				fieldVar := strings.ToLower(f.Name)
				fmt.Fprintf(out, "\tvar %s %s\n", fieldVar, f.Type)
			}
		}

		fmt.Fprint(out, `	if r.Method == http.MethodPost {
		body, err := bodyToMap(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

`)
		for _, p := range endpoint.Params {
			for _, f := range p.Fields {
				fieldVar := strings.ToLower(f.Name)
				fmt.Fprintf(out, "\t\t%s = body[%q]\n", fieldVar, f.ParamName())
			}
		}
		fmt.Fprint(out, "\t} else if r.Method == http.MethodGet {\n")
		fmt.Fprint(out, "\t\tquery := r.URL.Query()\n")
		for _, p := range endpoint.Params {
			for _, f := range p.Fields {
				fieldVar := strings.ToLower(f.Name)
				fmt.Fprintf(out, "\t\t%s = query.Get(%q)\n", fieldVar, fieldVar)
			}
		}
		fmt.Fprintln(out, "\t}")
	} else if endpoint.Method == http.MethodPost {
		fmt.Fprint(out, `	body, err := bodyToMap(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

`)

		for _, p := range endpoint.Params {
			for _, f := range p.Fields {
				fieldVar := strings.ToLower(f.Name)
				if f.Type == "string" {
					fmt.Fprintf(out, "\t%s := body[%q]\n", fieldVar, f.ParamName())
				} else if f.Type == "int" {
					format := `	%s, err := strconv.Atoi(body[%q])
	if err != nil {
		err := errors.New("%s must be int")
		writeResponse(w, nil, err, http.StatusBadRequest)
		return
	}
`
					fmt.Fprintf(out, format, fieldVar, fieldVar, fieldVar)
				}
			}
		}
	}

	fmt.Fprintln(out, "\tctx := r.Context()")

	for _, p := range endpoint.Params {
		fmt.Fprintf(out, "\tin := %s{\n", p.Name)
		for _, f := range p.Fields {
			fmt.Fprintf(out, "\t\t%s: %s,\n", f.Name, strings.ToLower(f.Name))
		}
		fmt.Fprintln(out, "\t}")
		validateMethod := `	if err := in.Validate(); err != nil {
		writeResponse(w, nil, err, http.StatusBadRequest)
		return
	}
	`
		fmt.Fprintln(out, validateMethod)
	}

	fmt.Fprintf(out, "\tresp, err := srv.%s(ctx, in)\n", endpoint.SeviceName)
	fmt.Fprintln(out, `	if err != nil {
		if e, ok := err.(ApiError); ok {
			writeResponse(w, nil, e.Err, e.HTTPStatus)
			return
		}
		writeResponse(w, nil, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, resp, nil, http.StatusOK)`)

	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)
}

func renderValidate(out *os.File, endpoint Endpoint) {
	for _, p := range endpoint.Params {
		fmt.Fprintf(out, "func (%s %s) Validate() error {\n", paramPrefix, p.Name)
		for _, f := range p.Fields {
			for _, v := range f.Validators {
				fmt.Fprintf(out, "\t%s\n", v.Stamp(f))
			}
		}
		fmt.Fprintln(out, "\treturn nil")
		fmt.Fprintln(out, "}")
	}
}
