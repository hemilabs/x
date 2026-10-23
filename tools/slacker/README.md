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
- `SLACK_URL`: [OPTIONAL] Slack API url for the client. Only useful for
  testing. It must use https, or http to a loopback address.

### Running

To use slacker, run:

```
slacker --channel <id> [--json | --raw] <message>
```

Slacker escapes markup in `<message>`, so a mention or a link shows as plain
text.  The `--raw` flag sends the markup as is.  Do not use `--raw` with text
from an untrusted source, such as a pull request title.

The `--json` flag will make `slacker` interpret the `<message>` as a JSON Block
Kit payload.  Slacker sends the payload as is, so do not build it from text
from an untrusted source.  For example:

```
slacker --channel ccc2124 --json '[{"type":"card","body":{"type": "mrkdwn","text":"My message here!"}}]'
```
