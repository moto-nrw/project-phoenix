// Per-Route-Bundle-Tabelle aus der Turbopack-Analyse (pnpm run perf:bundle-report,
// #2938). Das Einlesen der `analyze.data`-Köpfe liegt in bundle-routes.mjs und
// wird vom Bundle-Ratchet (#2939) mitbenutzt.
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { readRouteBundles } from "./bundle-routes.mjs";

const OUT_DIR = join(process.cwd(), "perf-results");
const OUT_FILE = join(OUT_DIR, "bundle.md");
const TOP_ROUTES = 15;
const TOP_PACKAGES = 8;

function kb(bytes) {
  return `${(bytes / 1024).toFixed(0)} kB`;
}

const sharedFiles = new Map();
const routes = readRouteBundles().map((r) => {
  for (const [filename, size] of r.fileSizes) {
    const entry = sharedFiles.get(filename) ?? { size, routes: 0 };
    entry.routes += 1;
    sharedFiles.set(filename, entry);
  }
  return {
    ...r,
    packages: [...r.packages.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, TOP_PACKAGES),
  };
});

routes.sort((a, b) => b.compressed - a.compressed);
const lines = [];
lines.push(
  `Routen analysiert: ${routes.length}. Größen = Client-JavaScript (\`/_next/static/chunks/*.js\`), komprimiert laut Turbopack.`,
);
lines.push("");
lines.push(
  "| Route | Client-JS (komprimiert) | roh | Chunks | größte Pakete (komprimiert) |",
);
lines.push("|---|---|---|---|---|");
for (const r of routes.slice(0, TOP_ROUTES)) {
  lines.push(
    `| \`${r.route}\` | ${kb(r.compressed)} | ${kb(r.raw)} | ${r.files} | ${r.packages.map(([p, s]) => `${p} ${kb(s)}`).join(", ")} |`,
  );
}
const perfTargets = [
  "/[tenant]/dashboard",
  "/[tenant]/active-supervisions",
  "/[tenant]/students/search",
  "/[tenant]/ogs-groups",
  "/[tenant]/time-tracking",
  "/[tenant]/betreuungsplan",
];
lines.push("");
lines.push("Die sechs gemessenen Screens:");
lines.push("");
lines.push(
  "| Route | Client-JS (komprimiert) | roh | Chunks | größte Pakete (komprimiert) |",
);
lines.push("|---|---|---|---|---|");
for (const target of perfTargets) {
  const r = routes.find((x) => x.route === target);
  if (!r) continue;
  lines.push(
    `| \`${r.route}\` | ${kb(r.compressed)} | ${kb(r.raw)} | ${r.files} | ${r.packages.map(([p, s]) => `${p} ${kb(s)}`).join(", ")} |`,
  );
}
lines.push("");
lines.push("Größte Chunks, die in vielen Routen stecken:");
lines.push("");
lines.push("| Chunk | komprimiert | Routen |");
lines.push("|---|---|---|");
for (const [filename, entry] of [...sharedFiles.entries()]
  .sort((a, b) => b[1].size - a[1].size)
  .slice(0, 12)) {
  lines.push(
    `| \`${filename.replace("[output]/.next/static/chunks/", "")}\` | ${kb(entry.size)} | ${entry.routes} |`,
  );
}
const report = lines.join("\n");
mkdirSync(OUT_DIR, { recursive: true });
writeFileSync(OUT_FILE, report);
process.stdout.write(`${report}\n`);
