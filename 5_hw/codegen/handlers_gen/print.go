package main

import (
	"os"
)



func printImports(out *os.File, packageName string) {
	out.WriteString("package " + packageName)
	out.WriteString("\n\n")
	out.WriteString(`import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
)`)
	out.WriteString("\n\n")
}

func printResultStruct(out *os.File) {
	out.WriteString("type result struct {\n")
	out.WriteString("\tError    string `json:\"error\"`\n")
	out.WriteString("\tResponse any    `json:\"response,omitempty\"`\n")
	out.WriteString("}\n\n")
}

func printCheckMethodMiddleware(out *os.File) {
	out.WriteString(`func checkMethodMiddleware(next http.HandlerFunc, allowed []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(allowed, r.Method) {
			writeResponse(w, nil, errors.New("bad method"), http.StatusNotAcceptable)
			return
		}

		next(w, r)
	}
}`)
	out.WriteString("\n")
}

func printAuthMiddleware(out *os.File) {
	out.WriteString("\n")
	out.WriteString(`func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth") != "100500" {
			writeResponse(w, nil, errors.New("unauthorized"), http.StatusForbidden)
			return
		}

		next(w, r)
	}
}`)
	out.WriteString("\n")
}

func printWriteResponse(out *os.File) {
	out.WriteString("\n")
	out.WriteString(`func writeResponse(w http.ResponseWriter, obj any, err error, status int) {
	resp := result{}
	if obj != nil {
		resp.Response = obj
	}
	if err != nil {
		resp.Error = err.Error()
	}

	if err := writeJSON(w, resp, status); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}`)
	out.WriteString("\n")
}

func printWriteJSON(out *os.File) {
	out.WriteString("\n")
	out.WriteString(`func writeJSON(w http.ResponseWriter, v any, status int) error {
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}`)
	out.WriteString("\n")
}

func printBodyToMap(out *os.File) {
	out.WriteString("\n")
	out.WriteString(`func bodyToMap(body io.ReadCloser) (map[string]string, error) {
	b, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	params := bytes.Split(b, []byte("&"))
	m := make(map[string]string, len(params))
	for _, pair := range params {
		kv := bytes.Split(pair, []byte("="))
		if len(kv) != 2 {
			continue
		}
		if len(kv[1]) == 0 {
			m[string(kv[0])] = ""
			continue
		}

		m[string(kv[0])] = string(kv[1])
	}

	return m, nil
}`)
	out.WriteString("\n")
}
