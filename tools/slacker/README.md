# Slacker

Slacker is a CLI tool to send messages to slack through an app.

## Build

```shell
cd tools/slacker

# Build the binary
go build -o ./cmd/slacker .
```

## Usage

### Env Variables

- `SLACK_OAUTH_TOKEN`: App OAuth Token
- `SLACK_URL`: [OPTIONAL] Slack url for the client. Only useful for testing.

### Running

To use slacker, run:

```
slacker --channel <id> [--json] <message>
```

The `--json` flag will make `slacker` interpret the `<message>` as a JSON Block
Kit payload. For example:

```
slacker --channel ccc2124 --json '[{"type":"card","body":{"type": "mrkdwn","text":"My message here!"}}]'
```
