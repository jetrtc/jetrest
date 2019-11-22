package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jetrtc/log"
	"github.com/jetrtc/rest"
)

func TestServer(t *testing.T) {
	log := log.NewSugar(log.NewLogger(log.GoLogger(log.Debug, os.Stderr, "", log.LstdFlags)))
	ts := httptest.NewServer(router(log))
	defer ts.Close()
	client := rest.NewClient(log, 5*time.Second)
	client.URL = ts.URL

	testREST(t, client, "GET", "/user/alice", "", http.StatusOK, `{"email":"alice@foo.com","display_name":"Alice"}`)
	testREST(t, client, "PUT", "/user/bob", "", http.StatusMethodNotAllowed, "")
	testREST(t, client, "GET", "/user/bob", "", http.StatusNotFound, "")
	testREST(t, client, "DELETE", "/user/bob", "", http.StatusNotFound, "")
	testREST(t, client, "POST", "/user/bob", ``, http.StatusBadRequest, "")
	testREST(t, client, "POST", "/user/bob", `{}`, http.StatusBadRequest, "")
	testREST(t, client, "POST", "/user/bob", `{"display_name":"Bob"}`, http.StatusBadRequest, "")
	testREST(t, client, "POST", "/user/bob", `{"email":"bob@bar.com","display_name":"Bob"}`, http.StatusOK, "")
	testREST(t, client, "GET", "/user/bob", "", http.StatusOK, `{"email":"bob@bar.com","display_name":"Bob"}`)
	testREST(t, client, "DELETE", "/user/bob", "", http.StatusOK, "")
	testREST(t, client, "GET", "/user/bob", "", http.StatusNotFound, "")
}

func testREST(t *testing.T, client *rest.Client, method, url, reqJson string, code int, expected string) {
	res, err := client.New(url).Do(method, []byte(reqJson))
	if err != nil {
		t.Fatal(err)
	}
	// Check the status code is what we expect.
	if res.StatusCode != code {
		t.Fatalf("%s %s returned wrong status code: got %v want %v", method, url, res.StatusCode, code)
	}
	if expected != "" {
		resJson := strings.Trim(string(res.Body), "\n")
		// Check the response body is what we expect.
		if resJson != expected {
			t.Fatalf("%s %s returned unexpected body: got %v want %v", method, url, resJson, expected)
		}
	}
}
