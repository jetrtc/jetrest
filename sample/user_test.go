package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServer(t *testing.T) {
	ts := httptest.NewServer(newHandler())
	defer ts.Close()

	testREST(t, "GET", fmt.Sprintf("%s/user/alice", ts.URL), "", http.StatusOK, `{"email":"alice@foo.com","display_name":"Alice"}`)
	testREST(t, "PUT", fmt.Sprintf("%s/user/bob", ts.URL), "", http.StatusMethodNotAllowed, "")
	testREST(t, "GET", fmt.Sprintf("%s/user/bob", ts.URL), "", http.StatusNotFound, "")
	testREST(t, "DELETE", fmt.Sprintf("%s/user/bob", ts.URL), "", http.StatusNotFound, "")
	testREST(t, "POST", fmt.Sprintf("%s/user/bob", ts.URL), ``, http.StatusBadRequest, "")
	testREST(t, "POST", fmt.Sprintf("%s/user/bob", ts.URL), `{}`, http.StatusBadRequest, "")
	testREST(t, "POST", fmt.Sprintf("%s/user/bob", ts.URL), `{"display_name":"Bob"}`, http.StatusBadRequest, "")
	testREST(t, "POST", fmt.Sprintf("%s/user/bob", ts.URL), `{"email":"bob@bar.com","display_name":"Bob"}`, http.StatusOK, "")
	testREST(t, "GET", fmt.Sprintf("%s/user/bob", ts.URL), "", http.StatusOK, `{"email":"bob@bar.com","display_name":"Bob"}`)
	testREST(t, "DELETE", fmt.Sprintf("%s/user/bob", ts.URL), "", http.StatusOK, "")
	testREST(t, "GET", fmt.Sprintf("%s/user/bob", ts.URL), "", http.StatusNotFound, "")
}

func testREST(t *testing.T, method, url, json string, code int, expected string) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(json)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	// Check the status code is what we expect.
	if status := res.StatusCode; status != code {
		t.Fatalf("%s %s returned wrong status code: got %v want %v", method, url, status, code)
	}
	if expected != "" {
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		json = strings.Trim(string(data), "\n")
		// Check the response body is what we expect.
		if json != expected {
			t.Fatalf("%s %s returned unexpected body: got %v want %v", method, url, json, expected)
		}
	}
}
