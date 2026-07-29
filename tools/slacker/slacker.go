// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/slack-go/slack"
)

type config struct {
	SlackOauthToken string
	SlackChannel    string
	SlackURL        string
}

func configFromEnv() (*config, error) {
	cfg := &config{
		SlackOauthToken: os.Getenv("SLACK_OAUTH_TOKEN"),
		SlackChannel:    os.Getenv("SLACK_CHANNEL"),
		SlackURL:        os.Getenv("SLACK_URL"),
	}

	if cfg.SlackOauthToken == "" {
		return nil, fmt.Errorf("SLACK_OAUTH_TOKEN must be set")
	}
	if cfg.SlackChannel == "" {
		return nil, fmt.Errorf("SLACK_CHANNEL must be set")
	}

	return cfg, nil
}

func newClient(cfg *config) *slack.Client {
	if cfg.SlackURL == "" {
		return slack.New(cfg.SlackOauthToken)
	}
	return slack.New(cfg.SlackOauthToken, slack.OptionAPIURL(cfg.SlackURL))
}

func sendText(client *slack.Client, channel, text string) error {
	_, _, err := client.PostMessage(channel, slack.MsgOptionText(text, false))
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
