// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		channel string
		url     string
		wantErr bool
	}{
		{
			name:    "missing token",
			token:   "",
			channel: "general",
			wantErr: true,
		},
		{
			name:    "missing channel",
			token:   "xoxb-test",
			channel: "",
			wantErr: true,
		},
		{
			name:    "valid",
			token:   "xoxb-test",
			channel: "general",
			url:     "http://testing.hemi.xyz/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SLACK_OAUTH_TOKEN", tt.token)
			t.Setenv("SLACK_URL", tt.url)

			cfg, err := configFromEnv(tt.channel)
			if err == nil {
				if tt.wantErr {
					t.Fatal("expected error")
				}
			} else {
				if !tt.wantErr {
					t.Fatal(err)
				}
				return
			}

			if cfg.SlackOauthToken != tt.token {
				t.Errorf("token: wanted %q, got %q", tt.token, cfg.SlackOauthToken)
			}
			if cfg.SlackChannel != tt.channel {
				t.Errorf("channel: wanted %q, got %q", tt.channel, cfg.SlackChannel)
			}
			if cfg.SlackURL != tt.url {
				t.Errorf("url: wanted %q, got %q", tt.url, cfg.SlackURL)
			}
		})
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return newCaptureServer(t, nil)
}

// newCaptureServer returns a test server that saves each request path and
// text form value in got, if got is not nil.
func newCaptureServer(t *testing.T, got *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			_ = r.ParseForm()
			*got = append(*got, r.URL.Path, r.FormValue("text"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"channel": "general",
			"ts":      "1234567890.123456",
		})
	}))
}

func TestSendText(t *testing.T) {
	s := newTestServer(t)
	defer s.Close()

	client, err := newClient(&config{
		SlackOauthToken: "xoxb-test",
		SlackURL:        s.URL + "/",
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := sendText(client, "general", "test message", false); err != nil {
		t.Fatal(err)
	}
}

func TestSendBlocks(t *testing.T) {
	s := newTestServer(t)
	defer s.Close()

	client, err := newClient(&config{
		SlackOauthToken: "xoxb-test",
		SlackURL:        s.URL + "/",
	})
	if err != nil {
		t.Fatal(err)
	}

	blocks := `[{"type":"section","text":{"type":"mrkdwn","text":"test message"}}]`
	if err := sendBlocks(client, "general", []byte(blocks)); err != nil {
		t.Fatalf("sendBlocks returned error: %v", err)
	}
}

func TestSendBlocksInvalidJSON(t *testing.T) {
	client, err := newClient(&config{
		SlackOauthToken: "xoxb-test",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = sendBlocks(client, "general", []byte("not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSendTextEscape(t *testing.T) {
	var got []string
	s := newCaptureServer(t, &got)
	defer s.Close()

	// No trailing slash: the client must add it.
	client, err := newClient(&config{SlackOauthToken: "xoxb-test", SlackURL: s.URL + "/api"})
	if err != nil {
		t.Fatal(err)
	}
	msg := "<!channel> <https://a.example|b.example>"
	if err := sendText(client, "general", msg, false); err != nil {
		t.Fatal(err)
	}
	if err := sendText(client, "general", msg, true); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/api/chat.postMessage", "&lt;!channel&gt; &lt;https://a.example|b.example&gt;",
		"/api/chat.postMessage", msg,
	}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func TestNewClientURL(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{url: "https://slack.example/api/"},
		{url: "http://127.0.0.1:8080/api/"},
		{url: "http://[::1]/api/"},
		{url: "http://localhost/api"},
		{url: "http://slack.example/api/", wantErr: true},
		{url: "ftp://slack.example/", wantErr: true},
		{url: "://bad", wantErr: true},
		{url: "http://localhost.evil.example/api/", wantErr: true},
		{url: "http://localhost@evil.example/api/", wantErr: true},
		{url: "https:///api/", wantErr: true},
		{url: "https:foo", wantErr: true},
		{url: "https://slack.example/api?x=1", wantErr: true},
		{url: "https://slack.example/api#x", wantErr: true},
	}
	for _, tt := range tests {
		_, err := newClient(&config{SlackOauthToken: "xoxb-test", SlackURL: tt.url})
		if (err != nil) != tt.wantErr {
			t.Errorf("%q: got %v, wantErr %v", tt.url, err, tt.wantErr)
		}
	}
}

func TestSendTextTimeout(t *testing.T) {
	done := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-done
	}))
	defer s.Close()
	defer close(done)

	old := clientTimeout
	clientTimeout = 100 * time.Millisecond
	defer func() { clientTimeout = old }()

	client, err := newClient(&config{SlackOauthToken: "xoxb-test", SlackURL: s.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := sendText(client, "general", "test", false); err == nil {
		t.Fatal("expected a timeout error")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("sendText took %v", d)
	}
}
