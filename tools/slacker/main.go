// Copyright (c) 2026 Hemi Labs, Inc.
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	jsonFlag := flag.Bool("json", false, "treat <message> as a JSON Block Kit payload")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		return fmt.Errorf("usage: %s [--json] <message>", os.Args[0])
	}
	message := args[0]

	cfg, err := configFromEnv()
	if err != nil {
		return err
	}

	client := newClient(cfg)

	if *jsonFlag {
		return sendBlocks(client, cfg.SlackChannel, []byte(message))
	}
	return sendText(client, cfg.SlackChannel, message)
}
