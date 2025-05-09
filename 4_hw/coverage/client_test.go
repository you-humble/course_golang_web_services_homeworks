package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type testCase struct {
	Title       string
	AccessToken string
	URL         string
	Request     SearchRequest
	Result      int
	IsError     bool
}

func brokenSearchServer(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")

	if query == "timeout" {
		time.Sleep(2 * time.Second)
	}

	if query == "internal" {
		w.WriteHeader(http.StatusInternalServerError)
		data, err := json.Marshal(SearchErrorResponse{
			Error: "internal server error",
		})
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		w.Write(data)
	}

	if query == "bad_request_unknown" {
		resp, _ := json.Marshal("UnknownError")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(resp)
		return
	}

	if query == "broken_json" {
		w.Write([]byte("UnknownError"))
		return
	}
}

func TestFindUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	brokenServer := httptest.NewServer(http.HandlerFunc(brokenSearchServer))
	defer brokenServer.Close()

	goodToken := "good token"

	testCases := []testCase{
		{
			Title:       "Simple success",
			AccessToken: goodToken,
			URL:         server.URL,
			Request: SearchRequest{
				Limit: 25,
			},
			Result:  25,
			IsError: false,
		},
		{
			Title:       "All params",
			AccessToken: goodToken,
			URL:         server.URL,
			Request: SearchRequest{
				Query:      "",
				OrderBy:    OrderByAsc,
				OrderField: "Id",
				Limit:      25,
				Offset:     1,
			},
			Result:  25,
			IsError: false,
		},
		{
			Title:       "Negative limit",
			AccessToken: goodToken,
			URL:         server.URL,
			Request: SearchRequest{
				Limit: -25,
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "Too big limit and negative offset",
			AccessToken: goodToken,
			URL:         server.URL,
			Request: SearchRequest{
				Limit:  26,
				Offset: -1,
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "client.Do unknown error",
			AccessToken: goodToken,
			URL:         "/error",
			Request:     SearchRequest{},
			Result:      0,
			IsError:     true,
		},
		{
			Title:       "client.Do timeout error",
			AccessToken: goodToken,
			URL:         brokenServer.URL,
			Request: SearchRequest{
				Query: "timeout",
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "Bad AccessToken",
			AccessToken: "bad token",
			URL:         server.URL,
			Request: SearchRequest{
				Query: "timeout",
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "SearchServer fatal error",
			AccessToken: goodToken,
			URL:         brokenServer.URL,
			Request: SearchRequest{
				Query: "internal",
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "SearchServer bad request",
			AccessToken: goodToken,
			URL:         server.URL,
			Request: SearchRequest{
				OrderBy: 3,
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "order_field is invalid",
			AccessToken: goodToken,
			URL:         server.URL,
			Request: SearchRequest{
				Limit:      25,
				OrderBy:    OrderByAsc,
				OrderField: "Gender",
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "cant unpack error json",
			AccessToken: goodToken,
			URL:         brokenServer.URL,
			Request: SearchRequest{
				Query: "bad_request_unknown",
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "cant unpack result json",
			AccessToken: goodToken,
			URL:         brokenServer.URL,
			Request: SearchRequest{
				Query: "broken_json",
			},
			Result:  0,
			IsError: true,
		},
		{
			Title:       "last test",
			AccessToken: goodToken,
			URL:         server.URL,
			Request: SearchRequest{
				Query:      "Sims",
				Offset:     0,
				Limit:      15,
				OrderBy:    OrderByAsc,
				OrderField: "Id",
			},
			Result:  1,
			IsError: false,
		},
	}

	for _, tc := range testCases {
		client := SearchClient{
			AccessToken: tc.AccessToken,
			URL:         tc.URL,
		}

		resp, err := client.FindUsers(tc.Request)

		if err != nil && !tc.IsError {
			t.Errorf("The test %q failed due to an error: %v", tc.Title, err)
		}

		if err == nil && tc.IsError {
			t.Errorf(
				"The test %q failed because an error was excepted, but got %v",
				tc.Title, err,
			)
		}

		if err != nil && tc.IsError {
			continue
		}

		if len(resp.Users) != tc.Result {
			t.Errorf(
				"The count of users in the test %q is not equal to the excepted count.\n\t%d != %d",
				tc.Title, len(resp.Users), tc.Result,
			)
		}
	}
}
