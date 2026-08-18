package devstack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testStack returns a Stack whose probes point at srv and whose timings are
// short enough to run in a unit test.
func testStack(t *testing.T, srv *httptest.Server) *Stack {
	t.Helper()
	_, port, ok := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	if !ok {
		t.Fatalf("unexpected test server URL %q", srv.URL)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("parse port from %q: %v", srv.URL, err)
	}
	return &Stack{cfg: Config{
		HorizonPort:  n,
		ReadyTimeout: 2 * time.Second,
		PollInterval: 5 * time.Millisecond,
		LogFile:      "compose.log",
		Stdout:       io.Discard,
		Stderr:       io.Discard,
	}}
}

func TestHorizonIngestedLedger(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    uint64
		wantErr string
	}{
		{name: "ingested", status: 200, body: `{"history_latest_ledger":42}`, want: 42},
		{name: "not yet ingested", status: 200, body: `{"history_latest_ledger":0}`, want: 0},
		{name: "field absent", status: 200, body: `{}`, want: 0},
		{name: "proxy up, horizon down", status: 502, body: `bad gateway`, wantErr: "status 502"},
		{name: "not json", status: 200, body: `<html>`, wantErr: "invalid character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer srv.Close()

			got, err := horizonIngestedLedger(context.Background(), srv.URL)
			switch {
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case got != tt.want:
				t.Fatalf("ledger = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestWaitForHorizonWaitsForIngestion pins the gate to the signal that
// matters: a horizon serving 200s with no history yet is not ready.
func TestWaitForHorizonWaitsForIngestion(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			fmt.Fprint(w, `{"history_latest_ledger":0}`)
			return
		}
		fmt.Fprint(w, `{"history_latest_ledger":7}`)
	}))
	defer srv.Close()

	if err := testStack(t, srv).waitForHorizon(context.Background()); err != nil {
		t.Fatalf("waitForHorizon: %v", err)
	}
	if got := calls.Load(); got < 3 {
		t.Fatalf("cleared after %d polls, want at least 3", got)
	}
}

func TestWaitUntilTimesOutNamingTheService(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"history_latest_ledger":0}`)
	}))
	defer srv.Close()

	s := testStack(t, srv)
	s.cfg.ReadyTimeout = 50 * time.Millisecond

	start := time.Now()
	err := s.waitForHorizon(context.Background())
	if err == nil {
		t.Fatal("expected timeout error")
	}
	for _, want := range []string{"horizon ingestion", "ledger 0", "compose.log"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want containing %q", err, want)
		}
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %s to give up on a 50ms budget", elapsed)
	}
}

// TestWaitUntilHonoursCallerDeadline covers the shared-budget contract:
// gates run under one ctx deadline, so a later gate inherits what earlier
// ones left rather than starting a fresh ReadyTimeout.
func TestWaitUntilHonoursCallerDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"history_latest_ledger":0}`)
	}))
	defer srv.Close()

	s := testStack(t, srv)
	s.cfg.ReadyTimeout = time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := s.waitForHorizon(ctx); err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("took %s; caller deadline of 50ms was ignored", elapsed)
	}
}

// TestWaitUntilReportsCallerCancellation distinguishes "you aborted" from
// "the stack is too slow" — only the latter should name a service.
func TestWaitUntilReportsCallerCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"history_latest_ledger":0}`)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := testStack(t, srv).waitForHorizon(ctx)
	if err != context.Canceled {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
