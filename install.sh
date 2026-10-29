#!/bin/sh
# VPS2Cloud installer: picks the build for this OS and CPU, downloads it from the GitHub release, checks its SHA-256
# against the release's checksums.txt, and installs it as /usr/local/bin/vps2cloud (or ~/.local/bin when not root).
#
#   curl -fsSL https://v2c.1ms.dog/i | sh                  install, then start the test with its menu
#   curl -fsSL https://v2c.1ms.dog/i | sh -s -- --no-run    install only
#   curl -fsSL https://v2c.1ms.dog/i | sh -s -- run [ARGS]  download to a temp dir, run once, delete
#
# Environment: V2C_VERSION (a tag such as v0.1.0; default latest), V2C_DIR (install directory),
# V2C_BASE (download base URL, for a mirror that copies the release files), V2C_NO_RUN=1 (install only).
set -eu

REPO="yt1988/vps2cloud"
VERSION="${V2C_VERSION:-latest}"
if [ "$VERSION" = latest ]; then
  BASE="${V2C_BASE:-https://github.com/$REPO/releases/latest/download}"
else
  BASE="${V2C_BASE:-https://github.com/$REPO/releases/download/$VERSION}"
fi

say() { printf '%s\n' "$*" >&2; }
die() { say "vps2cloud install: $*"; exit 1; }

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in linux|darwin|freebsd) ;; *) die "unsupported OS: $os (download a build from https://github.com/$REPO/releases)" ;; esac
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  armv7*|armv6*|armhf) arch=armv7 ;;
  i386|i686) arch=386 ;;
  riscv64) arch=riscv64 ;;
  mips|mipsel|mipsle)
    # OpenWrt and other MIPS routers: tell big from little endian by the ELF header of a system binary
    if [ "$(dd if=/bin/sh bs=1 skip=5 count=1 2>/dev/null | od -An -tx1 | tr -d ' ')" = 02 ]; then arch=mips; else arch=mipsle; fi ;;
  *) die "unsupported CPU: $arch" ;;
esac
file="vps2cloud-$os-$arch"

if command -v curl >/dev/null 2>&1; then fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then fetch() { wget -qO "$2" "$1"; }
else die "needs curl or wget"; fi

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t vps2cloud)
trap 'rm -rf "$tmp"' EXIT INT TERM
say "Downloading $file ($VERSION)…"
fetch "$BASE/$file" "$tmp/vps2cloud" || die "download failed: $BASE/$file"
fetch "$BASE/checksums.txt" "$tmp/checksums.txt" || die "download failed: $BASE/checksums.txt"

want=$(grep " $file\$" "$tmp/checksums.txt" | cut -d' ' -f1)
[ -n "$want" ] || die "no checksum for $file"
if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "$tmp/vps2cloud" | cut -d' ' -f1)
elif command -v shasum >/dev/null 2>&1; then got=$(shasum -a 256 "$tmp/vps2cloud" | cut -d' ' -f1)
elif command -v openssl >/dev/null 2>&1; then got=$(openssl dgst -sha256 "$tmp/vps2cloud" | sed 's/.*= //')
else die "needs sha256sum, shasum or openssl to verify the download"; fi
[ "$got" = "$want" ] || die "checksum mismatch for $file (expected $want, got $got)"
chmod +x "$tmp/vps2cloud"

if [ "${1:-}" = run ]; then
  shift
  # with no options it shows the menu, which reads the keyboard from /dev/tty (this script's input is the pipe)
  if [ $# -eq 0 ] && { : </dev/tty; } 2>/dev/null; then
    "$tmp/vps2cloud" </dev/tty
  else
    "$tmp/vps2cloud" "$@"
  fi
  exit $?
fi

if [ -n "${V2C_DIR:-}" ]; then dir="$V2C_DIR"
elif [ "$(id -u)" = 0 ] || [ -w /usr/local/bin ]; then dir=/usr/local/bin
else dir="$HOME/.local/bin"; fi
mkdir -p "$dir"
mv "$tmp/vps2cloud" "$dir/vps2cloud"
ln -sf "$dir/vps2cloud" "$dir/v2c" 2>/dev/null || true
say "Installed $("$dir/vps2cloud" --version) to $dir/vps2cloud (also as v2c)"
case ":$PATH:" in *":$dir:"*) ;; *) say "Add $dir to your PATH to run it by name." ;; esac
say "Run it again any time with: vps2cloud   (vps2cloud --help lists the options)"

# Start the test right away with its menu. Piped into sh, this script's input is the script itself, so the menu reads
# the keyboard from /dev/tty; without a terminal (CI, cron) it only installs.
if [ "${1:-}" != "--no-run" ] && [ -z "${V2C_NO_RUN:-}" ] && { : </dev/tty; } 2>/dev/null; then
  say ""
  exec "$dir/vps2cloud" </dev/tty
fi
