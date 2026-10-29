# Share link format

A share link is `https://v2c.1ms.dog/r#s=<data>`. Everything is in the URL fragment, which browsers do not send to a server, so the site never sees a result unless the viewer opens it. `<data>` is base64url without padding of a raw DEFLATE stream (`CompressionStream("deflate-raw")` in browsers).

The first decompressed byte tells the format apart. Format 1 is Me2Cloud's JSON (it starts with `{`). The CLI writes format 2:

| Field | Encoding |
|---|---|
| format | byte, `2` |
| flags | byte: bit 0 evening peak (20:00–23:00 local), bit 1 IPv6 only, bit 2 made by the CLI, bit 3 both IPv4 and IPv6 |
| mode | byte: `0` TCP handshake, `1` HTTPS, `2` ICMP |
| samples per region | uvarint |
| time | uvarint, minutes since the Unix epoch |
| endpoint list | 6 bytes: the first 6 bytes of SHA-256 over every region id of the list, in order, each followed by `\n` |
| from | three strings (city, country code, network name), each a uvarint byte length and UTF-8 |
| one block per IP family | IPv4 then IPv6 when bit 3 is set, otherwise the single family of bit 1 |
| · region count | uvarint, the number of regions in that list |
| · bitmap | ⌈count / 8⌉ bytes; bit *i* (least significant first) set when region *i* has a result |
| · per set bit, in list order | uvarint median × 10, uvarint (P90 − median) × 10, uvarint jitter × 10, byte loss % |

uvarint is the unsigned LEB128 encoding (Go's `binary.AppendUvarint`). Times are in milliseconds, so ×10 keeps 0.1 ms.

Regions are positions in an endpoint list, not names, so a viewer needs the list the link was made with. The site keeps every list it has served under `/e/<fingerprint>.json`, with the fingerprint in base64url (8 characters), and resolves a link with that file; the current list is usually the right one.

The IP address is never part of a link.
