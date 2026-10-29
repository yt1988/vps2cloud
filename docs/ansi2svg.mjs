#!/usr/bin/env node
// Turns a terminal recording (text with ANSI colors, e.g. from `script -qec 'vps2cloud …' /dev/null > run.txt`) into
// an SVG that looks like a terminal window, for the README.
// Usage: node docs/ansi2svg.mjs run.txt docs/demo.svg ["$ vps2cloud -p aws"]
import { readFileSync, writeFileSync } from "node:fs";

const [inFile, outFile, prompt] = process.argv.slice(2);
const COLORS = { 31: "#f87171", 32: "#4ade80", 33: "#facc15", 34: "#60a5fa", 35: "#e879f9", 36: "#22d3ee" };
const FG = "#e5e7eb", DIM = "#8b93a1", BG = "#111827", BAR = "#1f2937";
const CW = 8.4, LH = 20, PAD = 20, TOP = 44, SIZE = 14;   // cell width and line height in px

// East Asian wide characters take two cells, as in a terminal
const wide = ch => /[ᄀ-ᅟ⺀-꓏가-힣豈-﫿︰-﹏＀-｠￠-￦]/.test(ch);
const esc = s => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

let text = readFileSync(inFile, "utf8").replace(/\r/g, "").replace(/\n+$/, "");
if (prompt) text = "\x1b[2m$\x1b[0m " + prompt + "\n" + text;
const lines = text.split("\n");
let cols = 0;
const rows = lines.map(line => {
  // words with one style, each placed at its own column and given its exact width: viewers collapse spaces and use
  // their own fonts, so nothing may depend on either
  const out = [];
  let st = { bold: false, dim: false, color: null }, col = 0;
  for (const part of line.split(/(\x1b\[[0-9;]*m)/)) {
    const m = /^\x1b\[([0-9;]*)m$/.exec(part);
    if (m) {
      for (const c of (m[1] || "0").split(";").map(Number)) {
        if (c === 0) st = { bold: false, dim: false, color: null };
        else if (c === 1) st = { ...st, bold: true };
        else if (c === 2) st = { ...st, dim: true };
        else if (COLORS[c]) st = { ...st, color: COLORS[c] };
      }
      continue;
    }
    let buf = "", start = col, w = null;
    const flush = () => { if (buf) out.push({ col: start, text: buf, ...st }); buf = ""; w = null; };
    for (const ch of part) {
      const cw = wide(ch) ? 2 : 1;
      if (ch === " ") { flush(); col += 1; continue; }
      if (w !== null && cw !== w) flush();
      if (!buf) start = col;
      w = cw; buf += ch; col += cw;
    }
    flush();
  }
  cols = Math.max(cols, col);
  return out;
});

const W = Math.ceil(PAD * 2 + cols * CW), H = TOP + rows.length * LH + PAD - 4;
const body = rows.map((segs, i) => segs.map(s => {
  const fill = s.color || (s.dim ? DIM : FG), y = TOP + (i + 1) * LH - 6;
  const len = [...s.text].reduce((n, ch) => n + (wide(ch) ? 2 : 1), 0) * CW;
  return `<text x="${(PAD + s.col * CW).toFixed(1)}" y="${y}" fill="${fill}"${s.bold ? ' font-weight="700"' : ""} textLength="${len.toFixed(1)}" lengthAdjust="spacingAndGlyphs">${esc(s.text)}</text>`;
}).join("")).filter(Boolean).join("\n");

writeFileSync(outFile, `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" role="img" aria-label="vps2cloud output">
<rect width="${W}" height="${H}" rx="10" fill="${BG}"/>
<path d="M0 10a10 10 0 0 1 10-10h${W - 20}a10 10 0 0 1 10 10v18H0z" fill="${BAR}"/>
<circle cx="18" cy="14" r="5" fill="#ef4444"/><circle cx="34" cy="14" r="5" fill="#f59e0b"/><circle cx="50" cy="14" r="5" fill="#22c55e"/>
<g font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', 'Noto Sans Mono CJK SC', 'Microsoft YaHei UI', monospace" font-size="${SIZE}">
${body}
</g>
</svg>
`);
console.log(`${outFile}: ${W}×${H}, ${rows.length} lines`);
