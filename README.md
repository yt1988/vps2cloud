# VPS2Cloud

**English** · [简体中文](README.zh-CN.md)

Latency, jitter and loss from the machine you run it on to **361 regions of 17 cloud providers**: AWS, Azure, Google Cloud, Oracle, Alibaba Cloud, Tencent Cloud, Huawei Cloud, Akamai (Linode), DigitalOcean, Vultr, Hetzner, OVHcloud, IBM Cloud, Scaleway, Volcengine, Baidu AI Cloud and Naver Cloud.

One static binary, no dependencies. Run it on a VPS, a router or in CI to see which regions are close to it, then share the result as a link or paste it as a Markdown table. It speaks 10 languages.

It is the server-side companion of [Me2Cloud](https://m2c.1ms.dog), which runs the same test from your browser, and uses the same endpoint list.

```
$ vps2cloud --top 6
VPS2Cloud 0.2.0 · TCP handshake · 10 samples · IPv4 · 2026-10-29 04:09 UTC
from Nuremberg, DE · netcup GmbH (AS197540)

#  Provider      Region          City         Median  Min  P90  Jitter  Loss  Tier
1  Hetzner       nbg1            Nuremberg       0.7  0.6  0.9    ±0.2    0%  T0
2  Hetzner       fsn1            Falkenstein     3.0  2.8  3.1    ±0.1    0%  T0
3  Naver Cloud   de              Frankfurt       3.8  3.6  3.9    ±0.1    0%  T0
4  Vultr         fra             Frankfurt       3.8  3.7  4.0    ±0.2    0%  T0
5  DigitalOcean  fra1            Frankfurt       3.9  3.8  4.1    ±0.1    0%  T0
6  Oracle Cloud  eu-frankfurt-1  Frankfurt       4.1  3.9  4.2    ±0.1    0%  T0
… 355 more (--top 0 shows all)

361 of 361 regions answered in 1m13s
View and share: https://v2c.1ms.dog/r#s=…
```

## Install

```sh
curl -fsSL https://github.com/yt1988/vps2cloud/releases/latest/download/install.sh | sh
```

The script picks the build for your OS and CPU, downloads it from this repository's latest release, checks it against the release's `checksums.txt`, installs it as `/usr/local/bin/vps2cloud` (or `~/.local/bin` without root) with `v2c` as a short alias, and starts the test with its menu. Add `sh -s -- --no-run` (or set `V2C_NO_RUN=1`) to install only; without a terminal, as in CI, it only installs. Read it first if you like: it is [install.sh](install.sh).

Run once without installing:

```sh
curl -fsSL https://github.com/yt1988/vps2cloud/releases/latest/download/install.sh | sh -s -- run
```

Or download a binary from [Releases](https://github.com/yt1988/vps2cloud/releases): Linux (amd64, arm64, armv7, 386, riscv64, mips, mipsle for OpenWrt), macOS, FreeBSD and Windows.

## Usage

Run `vps2cloud` in a terminal with no options and it asks what to test, step by step, in color; Enter keeps each default and 0 goes back a step (0 on the first question exits):

```
What to test?            all regions · the 20 nearest · choose providers · choose continents · language
IP version?              IPv4 · IPv6 · both in one run (asked when the machine has IPv6)
How to measure?          TCP handshake · HTTPS · ICMP · all three
Samples per region?      5 · 10 · 20
```

It then prints the options it used, so the same test runs again without the menu. With options, or when the output is not a terminal, there is no menu:

```sh
vps2cloud -m tcp                   # every region, TCP handshake, 10 samples (any option skips the menu)
vps2cloud --near 10                # quick pass, then measure the 10 closest regions properly
vps2cloud -p aws,gcp -r 'eu-*'     # some providers, European regions
vps2cloud --continent as --top 10  # Asia, the 10 best
vps2cloud -m all -p hetzner        # TCP, HTTPS and ICMP side by side
vps2cloud -6                       # over IPv6 (regions without IPv6 are skipped and counted)
vps2cloud --ip both                # IPv4 and IPv6 in one run: one report with a part for each, one share link
vps2cloud --preset game            # mark regions that fit gaming (median ≤ 60 ms, jitter ≤ 10, no loss)
vps2cloud -o md                    # Markdown table for a forum post or an issue
vps2cloud -o json | jq '.regions[0]'
vps2cloud --max-ms 50              # exit status 3 when even the best region is above 50 ms
vps2cloud --list                   # providers; --list -p aws lists AWS's regions
```

`vps2cloud --help` lists every option.

### Language

The first time the menu runs it asks for a language (English, 简体中文, 繁體中文, 日本語, 한국어, Español, Français, Deutsch, Português, Русский) and remembers it in `~/.config/vps2cloud/config.json`. Change it later with the menu's last item, which is labelled in the current language and in English (for example `语言 / Language`), so you can always find your way back. For one run, use `--lang ja` or `V2C_LANG=ja`. The menu, the table and the Markdown follow the language; JSON and CSV keep English keys (each row names its `ip_family`). Colors appear only in a terminal; `--no-color` or `NO_COLOR` turns them off.

### What it measures

| Mode | What one sample is | Use it for |
|---|---|---|
| `tcp` (default) | A TCP handshake with the region's HTTPS port | Pure network round trip, no server work: the fairest comparison between regions |
| `https` | A request on an open, reused HTTPS connection, timed until the headers arrive | The same number Me2Cloud shows in a browser; includes a little server time |
| `icmp` | An ICMP echo | A classic ping. Some networks deprioritize ICMP, and Naver Cloud drops it |

Per region you get the median, minimum, P90, jitter (mean difference between consecutive samples), loss and a tier: T0 ≤ 30 ms, T1 ≤ 80, T2 ≤ 150, T3 ≤ 250, T4 above. The numbers are computed the same way as in Me2Cloud.

ICMP needs root, `CAP_NET_RAW` (`sudo setcap cap_net_raw+ep $(command -v vps2cloud)`) or a `net.ipv4.ping_group_range` that includes your group.

### Gentle on providers

Each host gets one sample at a time with a 250 ms pause in between, at most four hosts of one provider are measured at once, and a host is given up after three failures in a row. A default run sends each region ten handshakes.

## Privacy and what goes over the network

Besides the measurement itself, a run makes three requests:

- **The endpoint list** from `v2c.1ms.dog` (falling back to `m2c.1ms.dog`, then to the copy built into the binary). This request is how runs are counted: its User-Agent says `vps2cloud/<version> (<os>/<arch>[; ci])` and nothing else about the machine. `--offline` skips it.
- **Cloudflare's `/meta` and ident.me** to name the network the test runs from (city, provider, ASN) for the "from" line.

Results are never uploaded. The share link carries them in the URL fragment (`#…`), which browsers do not send to any server; it holds the per-region numbers and the "from" line, never the IP address. It is compact (about 2,000 characters for all 361 regions, a few hundred for 20; the format is in [docs/share-format.md](docs/share-format.md)). `--no-link` leaves it out. The IP is only printed with `--show-ip`.

## Endpoints

The list lives in [Me2Cloud](https://github.com/yt1988/me2cloud) (`public/endpoints.json`), where each endpoint is checked weekly. A workflow here copies it daily into [`endpoints.json`](endpoints.json), which is built into the binary. Use your own list with `--endpoints FILE`; the format is documented in Me2Cloud.

What it cannot do: it measures each provider's public API or storage endpoint in a region, not a virtual machine, and it does not measure bandwidth.

## Verify a download

Every release lists SHA-256 checksums and carries GitHub build provenance, so you can check that a binary was built from this repository by its release workflow:

```sh
gh attestation verify vps2cloud-linux-amd64 --repo yt1988/vps2cloud
```

## Build

```sh
go build -o vps2cloud .
go test ./...
```

Go 1.27 or later. `golang.org/x/net` (for ICMP) is the only dependency.

## License

MIT
