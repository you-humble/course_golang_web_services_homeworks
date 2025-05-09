package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

var (
	errWrongOrderField error = errors.New("ErrorBadOrderField")
	errWrongOrderBy    error = errors.New("wrong order by")
	errNotFound        error = errors.New("not found")
)

type Root struct {
	Rows []Row `xml:"row"`
}

type Row struct {
	ID        int    `xml:"id"`
	Age       int    `xml:"age"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Gender    string `xml:"gender"`
	About     string `xml:"about"`
}

func (r Row) toUser() User {
	return User{
		Id:     r.ID,
		Age:    r.Age,
		Name:   r.FirstName + " " + r.LastName,
		Gender: r.Gender,
		About:  r.About,
	}
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("AccessToken")
	if token != "good token" {
		writeErrorJSON(w, "Bad AccessToken", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("query")
	orderField := r.URL.Query().Get("order_field")
	orderByStr := r.URL.Query().Get("order_by")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	orderBy, err := strconv.Atoi(orderByStr)
	if err != nil {
		writeErrorJSON(w, "wrong request", http.StatusBadRequest)
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		writeErrorJSON(w, "wrong request", http.StatusBadRequest)
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		writeErrorJSON(w, "wrong request", http.StatusBadRequest)
		return
	}

	queryResult, err := find(query)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeUsersJSON(w, []User{})
			return
		}
		writeErrorJSON(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := order(queryResult, orderField, orderBy); err != nil {
		writeErrorJSON(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := paginate(queryResult, offset, limit)
	writeUsersJSON(w, result)
}

func find(query string) ([]User, error) {
	file, err := os.Open("dataset.xml")
	if err != nil {
		return []User{}, err
	}
	defer file.Close()

	var root Root
	decoder := xml.NewDecoder(file)
	if err := decoder.Decode(&root); err != nil {
		return []User{}, err
	}

	queryResult := make([]User, 0, len(root.Rows))
	for _, row := range root.Rows {
		u := row.toUser()
		if strings.Contains(u.Name, query) || strings.Contains(u.About, query) {
			queryResult = append(queryResult, u)
		}
	}

	if len(queryResult) == 0 {
		return []User{}, errNotFound
	}

	return queryResult, nil
}

func order(users []User, field string, by int) error {
	if field != "" && field != "Name" && field != "Id" && field != "Age" {
		return errWrongOrderField
	}

	if by == OrderByAsIs {
		return nil
	}

	if by == OrderByAsc {
		switch {
		case field == "" || field == "Name":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Name < users[j].Name
			})
		case field == "Id":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Id < users[j].Id
			})
		case field == "Age":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Age < users[j].Age
			})
		}
	} else if by == OrderByDesc {
		switch {
		case field == "" || field == "Name":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Name > users[j].Name
			})
		case field == "Id":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Id > users[j].Id
			})
		case field == "Age":
			sort.Slice(users, func(i, j int) bool {
				return users[i].Age > users[j].Age
			})
		}
	} else {
		return errWrongOrderBy
	}

	return nil
}

func paginate(users []User, offset, limit int) []User {
	if offset > len(users) {
		return []User{}
	}
	if limit == 0 {
		return users[offset:]
	}

	end := offset + limit
	if end > len(users) {
		end = len(users)
	}

	return users[offset:end]
}

func writeUsersJSON(w http.ResponseWriter, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		data, err := json.Marshal(SearchErrorResponse{
			Error: "internal server error",
		})
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		w.Write(data)
	}
}

func writeErrorJSON(w http.ResponseWriter, reason string, status int) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(SearchErrorResponse{
		Error: reason,
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		data, err := json.Marshal(SearchErrorResponse{
			Error: "internal server error",
		})
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		w.Write(data)
	}
}
