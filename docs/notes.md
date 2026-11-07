# Decisions and plans

Decisions that are not obvious from the code, and the plans not done yet.

## Decided

- **Timeouts are not packet loss (2026-11-04).** A TCP handshake or HTTPS request that does not finish in time may be a
  slow or blocking endpoint, so its column says "Timeouts" (超时率 and the other languages from the 1ms.dog glossary).
  With ICMP as the first mode an echo with no reply is a lost packet, and the column says "Loss". Field names
  (`loss_pct`, `--sort loss`, the share format) stayed, so old links and scripts keep working. Me2Cloud shows shared ICMP
  results as packet loss for the same reason.
- **No website of its own.** The VPS2Cloud site (v2c.1ms.dog) was taken down without a redirect. This is Me2Cloud's
  command-line version: m2c.1ms.dog serves the installers, the latest version and the release files, and share links
  open there. Short links made by early versions point at v2c.1ms.dog and give 404. Sending them on to m2c.1ms.dog
  would bring them back; the maintainer chose to leave them for now.
- **Release files through m2c.1ms.dog.** GitHub is often slow or unreachable from mainland China, so the update check,
  the installers and the downloads try m2c.1ms.dog first and fall back to GitHub. Every download is checked against the
  release's `checksums.txt`.
- **Short share links are stored on s.1ms.dog.** The default link is short, so the full link is kept there
  permanently. `--long-link` and `--offline` keep the result off any server.

## For later

- **Release check on a real server.** Before each release, run the candidate on the 1ms.dog server in Tokyo
  (TCP, and ICMP with sudo for that one run) and compare its medians with the Tokyo watcher's 24-hour medians. The steps
  are in Me2Cloud `server/watch/README.md`.
