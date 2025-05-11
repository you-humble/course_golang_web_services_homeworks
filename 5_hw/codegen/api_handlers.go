package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
)

type result struct {
	Error    string `json:"error"`
	Response any    `json:"response,omitempty"`
}

func (srv *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		handler := srv.handlerProfile
		handler = checkMethodMiddleware(
			handler, []string{
				http.MethodGet,
				http.MethodPost,
			})
		handler(w, r)
	case "/user/create":
		handler := srv.handlerCreate
		handler = checkMethodMiddleware(
			handler, []string{
				http.MethodPost,
			})
		handler = authMiddleware(handler)
		handler(w, r)
	default:
		writeResponse(w, nil, errors.New("unknown method"), http.StatusNotFound)
	}
}

func (srv *MyApi) handlerProfile(w http.ResponseWriter, r *http.Request) {
	var login string
	if r.Method == http.MethodPost {
		body, err := bodyToMap(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		login = body["login"]
	} else if r.Method == http.MethodGet {
		query := r.URL.Query()
		login = query.Get("login")
	}
	ctx := r.Context()
	in := ProfileParams{
		Login: login,
	}
	if err := in.Validate(); err != nil {
		writeResponse(w, nil, err, http.StatusBadRequest)
		return
	}
	
	resp, err := srv.Profile(ctx, in)
	if err != nil {
		if e, ok := err.(ApiError); ok {
			writeResponse(w, nil, e.Err, e.HTTPStatus)
			return
		}
		writeResponse(w, nil, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, resp, nil, http.StatusOK)
}

func (p ProfileParams) Validate() error {
	if len(p.Login) == 0 {
		return errors.New("login must me not empty")
	}
	return nil
}
func (srv *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	body, err := bodyToMap(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	login := body["login"]
	name := body["full_name"]
	status := body["status"]
	age, err := strconv.Atoi(body["age"])
	if err != nil {
		err := errors.New("age must be int")
		writeResponse(w, nil, err, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	in := CreateParams{
		Login: login,
		Name: name,
		Status: status,
		Age: age,
	}
	if err := in.Validate(); err != nil {
		writeResponse(w, nil, err, http.StatusBadRequest)
		return
	}
	
	resp, err := srv.Create(ctx, in)
	if err != nil {
		if e, ok := err.(ApiError); ok {
			writeResponse(w, nil, e.Err, e.HTTPStatus)
			return
		}
		writeResponse(w, nil, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, resp, nil, http.StatusOK)
}

func (p CreateParams) Validate() error {
	if len(p.Login) == 0 {
		return errors.New("login must me not empty")
	}
	if len(p.Login) < 10 {
		return errors.New("login len must be >= 10")
	}
	if p.Status == "" {
		p.Status = "user"
	}
	
	statusEnum := []string{"user", "moderator", "admin"}
	isCorrectStatus := false
	for _, v := range statusEnum {
		if p.Status == v {
			isCorrectStatus = true
			break
		}
	}
	if !isCorrectStatus {
		return errors.New("status must be one of [user, moderator, admin]")
	}
	
	if p.Age < 0 {
		return errors.New("age must be >= 0")
	}
	if p.Age > 128 {
		return errors.New("age must be <= 128")
	}
	return nil
}
func (srv *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		handler := srv.handlerCreate
		handler = checkMethodMiddleware(
			handler, []string{
				http.MethodPost,
			})
		handler = authMiddleware(handler)
		handler(w, r)
	default:
		writeResponse(w, nil, errors.New("unknown method"), http.StatusNotFound)
	}
}

func (srv *OtherApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	body, err := bodyToMap(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	username := body["username"]
	name := body["account_name"]
	class := body["class"]
	level, err := strconv.Atoi(body["level"])
	if err != nil {
		err := errors.New("level must be int")
		writeResponse(w, nil, err, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	in := OtherCreateParams{
		Username: username,
		Name: name,
		Class: class,
		Level: level,
	}
	if err := in.Validate(); err != nil {
		writeResponse(w, nil, err, http.StatusBadRequest)
		return
	}
	
	resp, err := srv.Create(ctx, in)
	if err != nil {
		if e, ok := err.(ApiError); ok {
			writeResponse(w, nil, e.Err, e.HTTPStatus)
			return
		}
		writeResponse(w, nil, err, http.StatusInternalServerError)
		return
	}

	writeResponse(w, resp, nil, http.StatusOK)
}

func (p OtherCreateParams) Validate() error {
	if len(p.Username) == 0 {
		return errors.New("username must me not empty")
	}
	if len(p.Username) < 3 {
		return errors.New("username len must be >= 3")
	}
	if p.Class == "" {
		p.Class = "warrior"
	}
	
	classEnum := []string{"warrior", "sorcerer", "rouge"}
	isCorrectClass := false
	for _, v := range classEnum {
		if p.Class == v {
			isCorrectClass = true
			break
		}
	}
	if !isCorrectClass {
		return errors.New("class must be one of [warrior, sorcerer, rouge]")
	}
	
	if p.Level < 1 {
		return errors.New("level must be >= 1")
	}
	if p.Level > 50 {
		return errors.New("level must be <= 50")
	}
	return nil
}
func checkMethodMiddleware(next http.HandlerFunc, allowed []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(allowed, r.Method) {
			writeResponse(w, nil, errors.New("bad method"), http.StatusNotAcceptable)
			return
		}

		next(w, r)
	}
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth") != "100500" {
			writeResponse(w, nil, errors.New("unauthorized"), http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func writeResponse(w http.ResponseWriter, obj any, err error, status int) {
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
}

func writeJSON(w http.ResponseWriter, v any, status int) error {
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

func bodyToMap(body io.ReadCloser) (map[string]string, error) {
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
}
