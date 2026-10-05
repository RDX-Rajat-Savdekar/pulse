package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestConsoleListsAnAcceptedEvent(t *testing.T) {
	h, stop, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	page, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(page.Body)
	page.Body.Close()
	if page.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Send event")) || !bytes.Contains(body, []byte(">Pulse<")) {
		t.Fatalf("page %d %s", page.StatusCode, body)
	}

	when := time.Now().UTC().Format(time.RFC3339)
	raw := `{"id":"evt-1","type":"page.view","source":"web","payload":{"path":"/"},"occurredAt":"` + when + `"}`
	res, err := http.Post(srv.URL+"/v1/events", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("accept %d", res.StatusCode)
	}

	again, err := http.Post(srv.URL+"/v1/events", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	dup, _ := io.ReadAll(again.Body)
	again.Body.Close()
	if again.StatusCode != http.StatusOK || !bytes.Contains(dup, []byte("duplicate")) {
		t.Fatalf("duplicate %d %s", again.StatusCode, dup)
	}

	query := `{"query":"{ events(limit: 5) { id type } }"}`
	res, err = http.Post(srv.URL+"/query", "application/json", strings.NewReader(query))
	if err != nil {
		t.Fatal(err)
	}
	listed, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !bytes.Contains(listed, []byte(`"id":"evt-1"`)) {
		t.Fatalf("tape %s", listed)
	}
}
