package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	emptyPath     string = "/"
	commandTable  string = "/table"
	commandRecord string = "/table/id"
	tableNotFound string = "unknown table"

	typeInt    string = "int"
	typeFloat  string = "float"
	typeString string = "string"
)

type response struct {
	Error    string `json:"error,omitempty"`
	Response any    `json:"response,omitempty"`
}

func writeResponse(w http.ResponseWriter, status int, resp response) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

type DbExplorer struct {
	db     *sql.DB
	Tables map[string]Table
}

type Table struct {
	Title  string
	Fields []Field
}
type Field struct {
	Title           string
	Type            string
	IsNullable      bool
	IsPrimary       bool
	IsAutoIncrement bool
}

func newField(info fieldRawInfo) Field {
	f := Field{Title: strings.ToLower(info.Title)}

	switch {
	case strings.HasPrefix(info.Type, "int"):
		f.Type = typeInt
	case strings.HasPrefix(info.Type, "float"):
		f.Type = typeFloat
	case strings.HasPrefix(info.Type, "varchar") || info.Type == "text":
		f.Type = typeString
	default:
		panic("unknown type: " + info.Type)
	}

	if info.IsNullable == "YES" {
		f.IsNullable = true
	}

	if info.IsPrimary == "PRI" {
		f.IsPrimary = true
	}

	if info.IsAutoIncrement == "auto_increment" {
		f.IsAutoIncrement = true
	}

	return f
}

type fieldRawInfo struct {
	Title           string
	Type            string
	IsNullable      string
	IsPrimary       string
	IsAutoIncrement string
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	de := DbExplorer{db: db, Tables: make(map[string]Table)}
	if err := de.setTables(); err != nil {
		return nil, fmt.Errorf("NewDbExplorer: %w", err)
	}
	return &de, nil
}

func (de *DbExplorer) setTables() error {
	rows, err := de.db.Query("SHOW TABLES;")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		t := Table{}
		if err := rows.Scan(&t.Title); err != nil {
			return err
		}

		de.Tables[t.Title] = t
	}

	if err := rows.Err(); err != nil {
		return err
	}

	for title, table := range de.Tables {
		fields, err := de.fieldsInfo(title)
		if err != nil {
			return err
		}
		table.Fields = fields
		de.Tables[title] = table
	}

	return nil
}

func (de *DbExplorer) fieldsInfo(tableTitle string) ([]Field, error) {
	rows, err := de.db.Query("SHOW FULL COLUMNS FROM " + tableTitle)
	if err != nil {
		return []Field{}, err
	}
	defer rows.Close()

	var fields []Field
	var skip interface{}
	for rows.Next() {
		info := fieldRawInfo{}
		if err := rows.Scan(
			&info.Title,
			&info.Type,
			&skip,
			&info.IsNullable,
			&info.IsPrimary,
			&skip,
			&info.IsAutoIncrement,
			&skip,
			&skip,
		); err != nil {
			return []Field{}, err
		}

		fields = append(fields, newField(info))
	}

	if err := rows.Err(); err != nil {
		return []Field{}, err
	}

	return fields, nil
}

func (de *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	const op = "ServeHTTP"
	command, req, err := de.QueryParams(r)
	if err != nil {
		internalError(w, op, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		switch command {
		case tableNotFound:
			writeResponse(w, http.StatusNotFound, response{Error: tableNotFound})
		case emptyPath:
			de.showTables(w)
		case commandTable:
			de.selectTable(w, req)
		case commandRecord:
			de.selectRecord(w, req)
		}
	case http.MethodPost:
		switch command {
		case commandRecord:
			de.updateRecord(w, req)
		}
	case http.MethodPut:
		switch command {
		case commandTable:
			de.insertRecord(w, req)
		}
	case http.MethodDelete:
		switch command {
		case commandRecord:
			de.deleteRecord(w, req)
		}
	default:
		http.Error(w, "bad request", http.StatusBadRequest)
	}
}

// * GET / - возвращает список все таблиц (которые мы можем использовать в дальнейших запросах)
func (de *DbExplorer) showTables(w http.ResponseWriter) {
	resp := map[string][]string{"tables": {}}
	for tableTitle := range de.Tables {
		resp["tables"] = append(resp["tables"], tableTitle)
	}
	sort.Strings(resp["tables"])

	writeResponse(w, http.StatusOK, response{Response: resp})
}

// * GET /$table?limit=5&offset=7 - возвращает список из 5 записей (limit) начиная с 7-й (offset) из таблицы $table. limit по-умолчанию 5, offset 0
func (de *DbExplorer) selectTable(w http.ResponseWriter, req request) {
	const op = "selectTable"

	table := de.Tables[req.TableTitle]
	query := fmt.Sprintf("SELECT * FROM %s LIMIT ?, ?", req.TableTitle)
	placeholders := []any{req.Offset, req.Limit}

	rows, err := de.db.Query(query, placeholders...)
	if err != nil {
		internalError(w, op, err)
		return
	}
	defer rows.Close()

	records := []map[string]any{}
	for rows.Next() {
		cols := make([]any, len(table.Fields))
		colsPtrs := make([]any, len(table.Fields))
		for i := 0; i < len(table.Fields); i++ {
			colsPtrs[i] = &cols[i]
		}

		if err := rows.Scan(colsPtrs...); err != nil {
			internalError(w, op, err)
			return
		}

		record := map[string]any{}
		for i, field := range table.Fields {
			record[field.Title] = convertVal(field, cols[i])
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		internalError(w, op, err)
		return
	}

	writeResponse(w, http.StatusOK, response{
		Response: map[string]any{"records": records},
	})
}

// * GET /$table/$id - возвращает информацию о самой записи или 404
func (de *DbExplorer) selectRecord(w http.ResponseWriter, req request) {
	const op = "selectRecord"

	table := de.Tables[req.TableTitle]

	var pri string
	for _, field := range table.Fields {
		if field.IsPrimary {
			pri = field.Title
		}
	}
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", req.TableTitle, pri)
	placeholders := []any{req.ID}

	cols := make([]any, len(table.Fields))
	colsPtrs := make([]any, len(table.Fields))
	for i := 0; i < len(table.Fields); i++ {
		colsPtrs[i] = &cols[i]
	}

	if err := de.db.QueryRow(query, placeholders...).Scan(colsPtrs...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeResponse(w, http.StatusNotFound, response{
				Error: "record not found",
			})
			return
		}
		internalError(w, op, err)
		return
	}

	record := map[string]any{}
	for i, field := range table.Fields {
		record[field.Title] = convertVal(field, cols[i])
	}

	writeResponse(w, http.StatusOK, response{
		Response: map[string]any{"record": record},
	})
}

// * PUT /$table - создаёт новую запись, данный по записи в теле запроса (POST-параметры)
func (de *DbExplorer) insertRecord(w http.ResponseWriter, req request) {
	const op = "insertRecord"

	table := de.Tables[req.TableTitle]

	var pri string
	fields := make([]string, 0, len(table.Fields))
	masks := make([]string, 0, len(table.Fields))
	placeholders := make([]any, 0, len(table.Fields))
	for _, field := range table.Fields {
		val, ok := req.Body[field.Title]
		if field.IsPrimary {
			pri = field.Title
			continue
		}

		if !ok {
			if field.IsNullable {
				val = nil
			} else {
				switch field.Type {
				case typeInt:
					val = 0
				case typeFloat:
					val = 0.0
				case typeString:
					val = ""
				}
			}
		}

		fields = append(fields, field.Title)
		placeholders = append(placeholders, val)
		masks = append(masks, "?")
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table.Title,
		strings.Join(fields, ", "),
		strings.Join(masks, ", "),
	)

	res, err := de.db.Exec(query, placeholders...)
	if err != nil {
		internalError(w, op, err)
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		internalError(w, op, err)
		return
	}

	writeResponse(w, http.StatusOK, response{
		Response: map[string]any{pri: id},
	})
}

// * POST /$table/$id - обновляет запись, данные приходят в теле запроса (POST-параметры)
func (de *DbExplorer) updateRecord(w http.ResponseWriter, req request) {
	const op = "updateRecord"

	table := de.Tables[req.TableTitle]

	var pri string
	fields := make([]string, 0, len(table.Fields))
	placeholders := make([]any, 0, len(table.Fields))
	for _, field := range table.Fields {
		val, ok := req.Body[field.Title]
		if field.IsPrimary {
			if ok {
				invalidTypeError(w, field.Title)
				return
			}
			pri = field.Title
			continue
		}

		if !ok {
			continue
		}

		if !validType(field, val) {
			invalidTypeError(w, field.Title)
			return
		}

		fields = append(fields, fmt.Sprintf("%s = ?", field.Title))
		placeholders = append(placeholders, val)
	}
	placeholders = append(placeholders, req.ID)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = ?",
		table.Title,
		strings.Join(fields, ", "),
		pri,
	)

	res, err := de.db.Exec(query, placeholders...)
	if err != nil {
		internalError(w, op, err)
		return
	}
	affected, err := res.RowsAffected()
	if err != nil {
		internalError(w, op, err)
		return
	}

	writeResponse(w, http.StatusOK, response{
		Response: map[string]any{"updated": affected},
	})
}

// * DELETE /$table/$id - удаляет запись
func (de *DbExplorer) deleteRecord(w http.ResponseWriter, req request) {
	const op = "deleteRecord"

	table := de.Tables[req.TableTitle]

	var pri string
	for _, field := range table.Fields {
		if field.IsPrimary {
			pri = field.Title
			break
		}
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", req.TableTitle, pri)
	placeholders := []any{req.ID}

	res, err := de.db.Exec(query, placeholders...)
	if err != nil {
		internalError(w, op, err)
		return
	}
	affected, err := res.RowsAffected()
	if err != nil {
		internalError(w, op, err)
		return
	}

	writeResponse(w, http.StatusOK, response{
		Response: map[string]any{"deleted": affected},
	})
}

type request struct {
	TableTitle string
	ID         string
	Offset     int
	Limit      int
	Body       map[string]any
}

func (de *DbExplorer) QueryParams(r *http.Request) (string, request, error) {
	var command string
	var req request

	if r.URL.Path == "/" {
		return emptyPath, req, nil
	}
	params := strings.FieldsFunc(r.URL.Path, func(r rune) bool {
		return r == '/'
	})

	if len(params) > 0 && len(params) <= 2 {
		title := params[0]
		if _, ok := de.Tables[title]; !ok {
			return tableNotFound, req, nil
		}
		req.TableTitle = title
		command = commandTable
		if len(params) == 2 {
			req.ID = params[1]
			command = commandRecord
		}
	} else {
		return "", req, fmt.Errorf("invalid path")
	}

	query := r.URL.Query()
	parseQueryInt := func(q url.Values, key string, def int) int {
		if val := q.Get(key); val != "" {
			if num, err := strconv.Atoi(val); err == nil {
				return num
			}
		}
		return def
	}

	req.Offset = parseQueryInt(query, "offset", 0)
	req.Limit = parseQueryInt(query, "limit", 5)

	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return "", request{}, err
		}
		defer r.Body.Close()

		if err := json.Unmarshal(b, &req.Body); err != nil {
			return "", request{}, err
		}
	}

	return command, req, nil
}

func internalError(w http.ResponseWriter, op string, err error) {
	log.Printf("%s - error: %+v", op, err)
	writeResponse(w, http.StatusInternalServerError, response{
		Error: "internal server error",
	})
}

func validType(field Field, val any) bool {
	switch val.(type) {
	case float64:
		if field.Type == "string" {
			return false
		}
	case string:
		if field.Type != "string" {
			return false
		}
	case nil:
		if !field.IsNullable {
			return false
		}
	default:
		return false
	}

	return true
}

func invalidTypeError(w http.ResponseWriter, fieldTitle string) {
	writeResponse(w, http.StatusBadRequest, response{
		Error: fmt.Sprintf("field %s have invalid type", fieldTitle),
	})
}

func convertVal(field Field, val any) any {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []byte:
		s := string(v)
		switch field.Type {
		case typeInt:
			if n, err := strconv.Atoi(s); err == nil {
				return n
			}
			return s
		case typeFloat:
			if n, err := strconv.ParseFloat(s, 64); err == nil {
				return n
			}
			return s
		default:
			return s
		}
	default:
		return v
	}
}
