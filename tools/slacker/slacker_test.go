// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	client := newClient(&config{
		SlackOauthToken: "xoxb-test",
		SlackURL:        s.URL + "/",
	})

	if err := sendText(client, "general", "test message"); err != nil {
		t.Fatal(err)
	}
}

func TestSendBlocks(t *testing.T) {
	s := newTestServer(t)
	defer s.Close()

	client := newClient(&config{
		SlackOauthToken: "xoxb-test",
		SlackURL:        s.URL + "/",
	})

	blocks := `[{"type":"section","text":{"type":"mrkdwn","text":"test message"}}]`
	if err := sendBlocks(client, "general", []byte(blocks)); err != nil {
		t.Fatalf("sendBlocks returned error: %v", err)
	}
}

func TestSendBlocksInvalidJSON(t *testing.T) {
	client := newClient(&config{
		SlackOauthToken: "xoxb-test",
	})

	err := sendBlocks(client, "general", []byte("not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}
