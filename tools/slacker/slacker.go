// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

type config struct {
	SlackOauthToken string
	SlackChannel    string
	SlackURL        string
}

func configFromEnv(channelID string) (*config, error) {
	cfg := &config{
		SlackOauthToken: os.Getenv("SLACK_OAUTH_TOKEN"),
		SlackChannel:    channelID,
		SlackURL:        os.Getenv("SLACK_URL"),
	}

	if cfg.SlackOauthToken == "" {
		return nil, fmt.Errorf("SLACK_OAUTH_TOKEN must be set")
	}
	if cfg.SlackChannel == "" {
		return nil, fmt.Errorf("channel ID must be set")
	}

	return cfg, nil
}

// clientTimeout limits each request to Slack.
var clientTimeout = 30 * time.Second

func newClient(cfg *config) (*slack.Client, error) {
	opts := []slack.Option{
		slack.OptionHTTPClient(&http.Client{Timeout: clientTimeout}),
	}
	if cfg.SlackURL != "" {
		u, err := url.Parse(cfg.SlackURL)
		if err != nil {
			return nil, fmt.Errorf("SLACK_URL: %w", err)
		}
		// slack-go adds the method name to the URL as a string, so a
		// query or fragment would take the method name.
		if u.Host == "" || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("SLACK_URL must be a host and path, with no query or fragment")
		}
		// The token goes in each request, so require https.  Allow
		// http only to a loopback address, for tests.
		if u.Scheme != "https" && (u.Scheme != "http" || !isLoopback(u.Hostname())) {
			return nil, errors.New("SLACK_URL must use https, or http to a loopback address")
		}
		apiURL := strings.TrimSuffix(cfg.SlackURL, "/") + "/"
		opts = append(opts, slack.OptionAPIURL(apiURL))
	}
	return slack.New(cfg.SlackOauthToken, opts...), nil
}

// isLoopback reports whether host is localhost or a loopback IP address.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sendText posts text.  Unless raw is set, it escapes markup, so a
// mention or link in text shows as plain text.
func sendText(client *slack.Client, channel, text string, raw bool) error {
	_, _, err := client.PostMessage(channel, slack.MsgOptionText(text, !raw))
	return err
}

func sendBlocks(client *slack.Client, channel string, data []byte) error {
	var blocks slack.Blocks
	if err := json.Unmarshal(data, &blocks); err != nil {
		return fmt.Errorf("unmarshal blocks: %w", err)
	}

	_, _, err := client.PostMessage(channel, slack.MsgOptionBlocks(blocks.BlockSet...))
	return err
}
