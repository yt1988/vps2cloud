# Design notes

Technical reasons behind choices the code does not explain.

- **Timeouts are not packet loss.** A TCP handshake or HTTPS request that does not finish in time may be a slow or
  blocking endpoint, so its column says "Timeouts" (超时率, and the other languages from the same glossary). With ICMP
  as the first mode, an echo with no reply is a lost packet, and the column says "Loss". Field names (`loss_pct`,
  `--sort loss`, the share format) did not change, so old links and scripts keep working. Me2Cloud shows shared ICMP
  results as packet loss for the same reason.
- **Release files through m2c.1ms.dog.** GitHub is often slow or unreachable from mainland China. So the update check,
  the installers and the downloads try m2c.1ms.dog first, and fall back to GitHub. Every download is checked against
  the release's `checksums.txt` either way.
- **Short share links are stored.** The default link is short, so the full link is kept on s.1ms.dog permanently. The
  same result always gets the same short link. `--long-link` and `--offline` keep the result off any server: the full
  link carries everything in the URL fragment, which browsers do not send.
- **Gentle on providers.** At most four hosts of one provider are measured at once, each one sample at a time with a
  pause. Every host is a separate regional server, so this is as light per host as Me2Cloud in a browser (two at
  once), and a full run still takes about a minute.
