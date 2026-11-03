// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/pflag"
)

const slackerHelp = `Slacker is a CLI tool to send messages to slack through an app.

Usage:
  slacker --channel <id> [--json | --raw] <message>

Environment Variables:

[SLACK_OAUTH_TOKEN]      (Required) App OAuth Token.
[SLACK_URL]              (Optional) Slack API url, https or loopback http. Useful for testing.

Flags:`

// sendTimeout limits the time to send the message to Slack.
const sendTimeout = 30 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	jsonFlag := pflag.Bool("json", false, "treat <message> as a JSON Block Kit payload, sent as is")
	rawFlag := pflag.Bool("raw", false, "do not escape markup such as mentions and links in <message>")
	channelID := pflag.StringP("channel", "c", "", "slack channel id to send the message in")
	pflag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", slackerHelp)
		pflag.PrintDefaults()
	}
	pflag.Parse()

	args := pflag.Args()
	if len(args) != 1 {
		pflag.Usage()
		return errors.New("expected exactly one <message> argument")
	}
	message := args[0]

	cfg, err := configFromEnv(*channelID)
	if err != nil {
		pflag.Usage()
		return err
	}

	client, err := newClient(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	if *jsonFlag {
		return sendBlocks(ctx, client, cfg.SlackChannel, []byte(message))
	}
	return sendText(ctx, client, cfg.SlackChannel, message, *rawFlag)
}
