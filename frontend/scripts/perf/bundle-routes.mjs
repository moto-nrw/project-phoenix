// Client-Bundle je Route aus der Turbopack-Analyse (#2938, #2939). Liest den
// JSON-Kopf jeder `.next/diagnostics/analyze/data/**/analyze.data` (4-Byte-
// Big-Endian-Länge + JSON, danach Binär-Indizes, die hier nicht gebraucht
// werden) und summiert die Client-Chunks (`/static/chunks/`) je Route und je
// Paket. Das Format ist als experimentell markiert; `assertHeader` bricht laut
// ab, wenn sich der Kopf ändert, damit der Ratchet nie still grün wird.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const DATA_DIR = join(process.cwd(), ".next", "diagnostics", "analyze", "data");

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
  const header = JSON.parse(buffer.subarray(4, 4 + length).toString("utf8"));
  assertHeader(header, file);
  return header;
}

/** @param {unknown} header @param {string} file */
function assertHeader(header, file) {
  const ok =
    header &&
    Array.isArray(header.output_files) &&
    Array.isArray(header.chunk_parts) &&
    Array.isArray(header.sources) &&
    header.output_files.every((f) => typeof f?.filename === "string") &&
    header.chunk_parts.every(
      (p) =>
        Number.isInteger(p?.output_file_index) &&
        Number.isInteger(p?.source_index) &&
        typeof p?.size === "number" &&
        typeof p?.compressed_size === "number",
    ) &&
    header.sources.every((s) => typeof s?.path === "string");
  if (!ok) {
    throw new Error(
      `Unbekanntes analyze.data-Format in ${file}. \`next experimental-analyze\` hat seinen Kopf geändert; scripts/perf/bundle-routes.mjs anpassen, bevor der Ratchet weiterläuft.`,
    );
  }
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

/**
 * @typedef {{ route: string, files: number, raw: number, compressed: number,
 *   packages: Map<string, number>, fileSizes: Map<string, number> }} RouteBundle
 */

/** Seitenrouten (ohne API, Middleware, Instrumentation). @returns {RouteBundle[]} */
export function readRouteBundles(dataDir = DATA_DIR) {
  const routes = [];
  for (const file of findAnalyzeFiles(dataDir)) {
    const route = `/${relative(dataDir, file).replace(/\/?analyze\.data$/, "")}`;
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
      const pkg = packageOf(
        sourcePath(header.sources, part.source_index, cache),
      );
      packages.set(pkg, (packages.get(pkg) ?? 0) + part.compressed_size);
      const filename = header.output_files[part.output_file_index].filename;
      fileSizes.set(
        filename,
        (fileSizes.get(filename) ?? 0) + part.compressed_size,
      );
    }
    routes.push({
      route,
      files: clientFiles.size,
      raw,
      compressed,
      packages,
      fileSizes,
    });
  }
  return routes;
}
