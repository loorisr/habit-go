package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckHealth(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	if err := checkHealth(ok.URL + "/api/health"); err != nil {
		t.Errorf("healthy server: %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	if err := checkHealth(bad.URL + "/api/health"); err == nil {
		t.Error("500 reported as healthy")
	}

	if err := checkHealth("http://127.0.0.1:1/api/health"); err == nil {
		t.Error("unreachable server reported as healthy")
	}
}
