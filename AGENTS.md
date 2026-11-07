# AGENTS.md

For anyone, human or AI agent, who maintains this repository. The README is for users; this file is for working on
the code. The operations notes for 1ms.dog itself (accounts, the server, other tools) are in the private repository
1ms-shell.

## What this is

vps2cloud is the command-line version of [Me2Cloud](https://github.com/yt1988/me2cloud). One static Go binary measures
latency, jitter and timeouts from the machine it runs on to the 392 regions of 22 cloud providers. It can measure by
TCP handshake (the default), HTTPS request or ICMP echo. Results show as a table, or as Markdown, JSON or CSV, with a
share link that opens on m2c.1ms.dog.

- `main.go`: options and help. `run.go`: measuring and the statistics. `probe.go`: the three ways of measuring.
- `output.go`: table, Markdown, JSON and CSV. `share.go`: the share link (format in `docs/share-format.md`).
- `i18n.go`: text in 10 languages; option descriptions in the help stay English.
- `interactive.go`: the wizard when run with no options in a terminal. `progress.go`, `stream.go`: live progress.
- `proxy.go`: notices when a local proxy or TUN answers instead of the real host (fake-ip addresses).
- `update.go`: the hourly update check and self-update. `endpoints.go`: the built-in endpoint list (`endpoints.json`).
- `install.sh`, `install.ps1`: the installers that m2c.1ms.dog serves at `/i` and `/i.ps1`.
- `docs/ansi2svg.mjs`: turns a terminal recording into the README's `docs/demo*.svg`.

The endpoint list is not made here. `.github/workflows/endpoints.yml` copies Me2Cloud's `public/endpoints.json` into
`endpoints.json` daily. Change endpoints in Me2Cloud.

## Run it locally

Go 1.27 or later (the version in `go.mod`).

```bash
go build -o vps2cloud .
./vps2cloud -p aws,hetzner -n 3            # a short real run
./vps2cloud --offline -p aws -o json --stdout | head
```

A development build skips the update check. Set `V2C_NO_UPDATE=1` to skip it in any build. On a machine behind a
fake-ip proxy, the numbers are the distance to the proxy; the tool says so.

## Test

CI (`.github/workflows/ci.yml`) runs all of this on every push and pull request:

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
./vps2cloud --offline -p aws,hetzner,vultr -n 3 -m tcp,https --top 10
```

Pick by what you changed:

- **Text**: `TestTranslationsComplete` fails when a language lacks a key. Read the table in a narrow terminal, since a
  longer header widens every row.
- **Output formats or the share link**: `share_test.go`. Open a `--long-link` link on m2c.1ms.dog and check it reads
  the same as the table. Old links must keep opening, so the format only grows (see `docs/share-format.md`).
- **Measuring**: run against a few providers with each mode (`-m tcp`, `-m https`; `-m icmp` needs root or
  `CAP_NET_RAW`). Before a release, 1ms.dog also runs the candidate once on its Tokyo server and compares with the
  Tokyo watcher (Me2Cloud `server/watch/README.md`).
- **Updates**: `update_test.go`.

## Release

A pushed tag `vX.Y.Z` runs `.github/workflows/release.yml`:

- It builds 12 targets and writes `checksums.txt`.
- It attests build provenance and publishes the GitHub release with the installers.
- m2c.1ms.dog then serves the new version at `/v` and the files at `/dl/<tag>/…` (Me2Cloud `src/worker.js`). Installed
  copies offer the update within an hour.

Releases are made only by the maintainer. Do not push a tag on your own. Pushing to `main` releases nothing.

After a release, check:

- `curl -s https://m2c.1ms.dog/v` shows the new version.
- `curl -fsSL https://m2c.1ms.dog/i | sh` installs it.
- `vps2cloud --version` prints it.

## Things that are easy to break

- **Share links**: field names and the binary format stay compatible (`loss_pct`, `--sort loss`, the share format),
  even when the visible words change.
- **Privacy**: share links never carry the IP address. `--show-ip` is the only way it is printed.
  - The only requests besides measuring are the ones listed in the README's privacy section.
  - The User-Agent on the endpoint list request is how runs are counted, and says nothing else about the machine.
- **Words**: a TCP or HTTPS sample with no answer in time is a timeout ("Timeouts" column). Only with ICMP as the first
  mode is it packet loss ("Loss"). See `docs/notes.md`.
- **Gentle on providers**: at most four hosts of one provider at once, one sample at a time per host with a pause.
  Do not raise these without a reason; it is part of the README's promise.

Decisions and plans the code does not show go in `docs/notes.md`.
