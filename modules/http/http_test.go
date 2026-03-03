package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	ts := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(200)
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	// For tests, bypass the safe dialer entirely and use a plain client
	// pointed at the test server.
	saved := client
	client = ts.Client()
	client.Timeout = 10 * time.Second
	defer func() { client = saved }()

	resp, err := Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Body != "hello" {
		t.Errorf("expected 'hello', got %q", resp.Body)
	}
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		w.WriteHeader(201)
		w.Write([]byte("created"))
	}))
	defer ts.Close()

	saved := client
	client = ts.Client()
	client.Timeout = 10 * time.Second
	defer func() { client = saved }()

	resp, err := Post(ts.URL, "application/json", `{"key":"value"}`)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	if resp.Body != "created" {
		t.Errorf("expected 'created', got %q", resp.Body)
	}
}

func TestDo(t *testing.T) {
	ts := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if v := r.Header.Get("X-Custom"); v != "val" {
			t.Errorf("expected X-Custom: val, got %q", v)
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	saved := client
	client = ts.Client()
	client.Timeout = 10 * time.Second
	defer func() { client = saved }()

	resp, err := Do(Request{
		Method:  "PUT",
		URL:     ts.URL,
		Headers: map[string][]string{"X-Custom": {"val"}},
		Body:    "data",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestDoEmptyMethodDefaultsToGET(t *testing.T) {
	ts := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(200)
	}))
	defer ts.Close()

	saved := client
	client = ts.Client()
	client.Timeout = 10 * time.Second
	defer func() { client = saved }()

	_, err := Do(Request{URL: ts.URL})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTimeout(t *testing.T) {
	ts := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	saved := client
	c := ts.Client()
	c.Timeout = 100 * time.Millisecond
	client = c
	defer func() { client = saved }()

	_, err := Get(ts.URL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout-related error, got: %v", err)
	}
}

func TestInvalidURL(t *testing.T) {
	_, err := Get("://bad")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
