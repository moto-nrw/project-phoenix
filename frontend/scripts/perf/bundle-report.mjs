// Per-Route-Bundle-Tabelle aus der Turbopack-Analyse (pnpm run perf:bundle-report,
// #2938). Liest den JSON-Kopf jeder `.next/diagnostics/analyze/data/**/analyze.data`
// (4-Byte-Big-Endian-Länge + JSON, danach Binär-Indizes, die hier nicht gebraucht
// werden) und summiert die Client-Chunks (`/static/chunks/`) je Route und je Paket.
import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";

const DATA_DIR = join(process.cwd(), ".next", "diagnostics", "analyze", "data");
const OUT_FILE = join(process.cwd(), "perf-results", "bundle.md");
const TOP_ROUTES = 15;
const TOP_PACKAGES = 8;

/** @param {string} dir @returns {string[]} */
function findAnalyzeFiles(dir) {
  const out = [];
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) out.push(...findAnalyzeFiles(path));
    else if (entry === "analyze.data") out.push(path);
  }
  return out;
}

/** @param {string} file */
function readHeader(file) {
  const buffer = readFileSync(file);
  const length = buffer.readUInt32BE(0);
  return JSON.parse(buffer.subarray(4, 4 + length).toString("utf8"));
}

/** Vollständiger Quellpfad über die parent_source_index-Kette. */
function sourcePath(sources, index, cache) {
  if (cache.has(index)) return cache.get(index);
  const source = sources[index];
  const parent = source.parent_source_index;
  const path =
    parent === undefined || parent === null || parent === index
      ? source.path
      : `${sourcePath(sources, parent, cache)}/${source.path}`;
  cache.set(index, path);
  return path;
}

/** `node_modules/.pnpm/<name>@<ver>_.../node_modules/<name>/...` -> `<name>` */
function packageOf(rawPath) {
  // Die Segmente tragen teils eigene Schrägstriche; erst normalisieren.
  const path = rawPath.replace(/\/+/g, "/");
  const match = /node_modules\/((?:@[^/]+\/)?[^/]+)\/(?!.*node_modules\/)/.exec(
    path,
  );
  if (match) return match[1];
  return path.includes("node_modules")
    ? "(node_modules, sonstige)"
    : "(eigener Code)";
}

function kb(bytes) {
  return `${(bytes / 1024).toFixed(0)} kB`;
}

const routes = [];
const sharedFiles = new Map();
for (const file of findAnalyzeFiles(DATA_DIR)) {
  const route = `/${relative(DATA_DIR, file).replace(/\/?analyze\.data$/, "")}`;
  if (
    route.startsWith("/api/") ||
    route === "/middleware" ||
    route.startsWith("/instrumentation")
  )
    continue;
  const header = readHeader(file);
  const clientFiles = new Set();
  header.output_files.forEach((entry, index) => {
    if (/\/static\/chunks\/.*\.js$/.test(entry.filename))
      clientFiles.add(index);
  });
  const cache = new Map();
  const packages = new Map();
  const fileSizes = new Map();
  let raw = 0;
  let compressed = 0;
  for (const part of header.chunk_parts) {
    if (!clientFiles.has(part.output_file_index)) continue;
    raw += part.size;
    compressed += part.compressed_size;
    const pkg = packageOf(sourcePath(header.sources, part.source_index, cache));
    packages.set(pkg, (packages.get(pkg) ?? 0) + part.compressed_size);
    const filename = header.output_files[part.output_file_index].filename;
    fileSizes.set(
      filename,
      (fileSizes.get(filename) ?? 0) + part.compressed_size,
    );
  }
  for (const [filename, size] of fileSizes) {
    const entry = sharedFiles.get(filename) ?? { size, routes: 0 };
    entry.routes += 1;
    sharedFiles.set(filename, entry);
  }
  routes.push({
    route,
    files: clientFiles.size,
    raw,
    compressed,
    packages: [...packages.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, TOP_PACKAGES),
  });
}

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
writeFileSync(OUT_FILE, report);
process.stdout.write(`${report}\n`);
