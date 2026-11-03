import { readFileSync } from "node:fs";

export const CEILINGS_FORMAT = "moto-bundle-ceilings/v1";
// A PR label, read fresh from the GitHub API on every (re-)run, approves one
// PR's growth. The reason belongs in the PR description.
export const GROWTH_APPROVAL_LABEL = "bundle-growth-approved";
const TOLERANCE_RATIO = 0.02;
const TOLERANCE_BYTES = 10 * 1024;
// A merge-base this close to a ceiling must not turn an unrelated PR red
// (#5396: 4 of 5 bundle failures were 13 to 201 bytes over). Such PRs are
// judged by their own delta; development pushes, the nightly and release PRs
// still enforce the ceiling.
export const NEAR_CEILING_BYTES = 1024;

/** Growth a PR may add to one route without approval. */
export function deltaTolerance(baseBytes) {
  return Math.max(TOLERANCE_BYTES, Math.round(baseBytes * TOLERANCE_RATIO));
}

export function validBytes(value) {
  return Number.isSafeInteger(value) && value >= 0;
}

/** Absolute ceilings for a few heavily used routes. Missing or malformed fails closed. */
export function readCeilings(file) {
  const policy = JSON.parse(readFileSync(file, "utf8"));
  if (
    policy?.format !== CEILINGS_FORMAT ||
    !policy.routes ||
    typeof policy.routes !== "object" ||
    Array.isArray(policy.routes)
  ) {
    throw new Error(`Invalid bundle ceiling policy: ${file}`);
  }
  const ceilings = new Map();
  for (const [route, maxBytes] of Object.entries(policy.routes)) {
    if (!route.startsWith("/") || !validBytes(maxBytes))
      throw new Error(`Invalid bundle ceiling for ${route} in ${file}`);
    ceilings.set(route, maxBytes);
  }
  if (ceilings.size === 0)
    throw new Error(`Bundle ceiling policy contains no routes: ${file}`);
  return ceilings;
}

const GROWTH_HINT = `Bundle verkleinern (Lazy Loading, leichtere Bibliothek) oder, wenn das Feature den Zuwachs braucht, Label \`${GROWTH_APPROVAL_LABEL}\` setzen, im PR-Text begründen und die fehlgeschlagenen Jobs neu starten.`;

/**
 * Feature PRs: each route's own delta against the merge-base is the gate; the
 * approval label turns delta failures into warnings. Ceilings apply to every
 * run and are not waived by the label.
 */
export function compareBundles(
  ceilings,
  measured,
  base = null,
  { growthApproved = false } = {},
) {
  if (!(measured instanceof Map) || measured.size === 0)
    throw new Error("No measured bundle routes");
  const failures = [];
  const warnings = [];
  const rows = [];
  for (const [route, bytes] of [...measured.entries()].sort(([a], [b]) =>
    a < b ? -1 : a > b ? 1 : 0,
  )) {
    if (!validBytes(bytes))
      throw new Error(`Invalid measured bundle bytes for ${route}`);
    const baseBytes = base?.get(route);
    if (baseBytes !== undefined && !validBytes(baseBytes))
      throw new Error(`Invalid base bundle bytes for ${route}`);
    const maxBytes = ceilings.get(route);
    rows.push({
      route,
      bytes,
      baseBytes,
      maxBytes,
      delta: baseBytes === undefined ? null : bytes - baseBytes,
    });
    if (baseBytes !== undefined && bytes - baseBytes > deltaTolerance(baseBytes)) {
      const message = `${route}: PR-Zuwachs ${bytes - baseBytes} Bytes überschreitet die Toleranz ${deltaTolerance(baseBytes)} Bytes gegenüber der Merge-Basis (${baseBytes} Bytes).`;
      if (growthApproved)
        warnings.push(`${message} Freigegeben mit \`${GROWTH_APPROVAL_LABEL}\`.`);
      else failures.push(`${message} ${GROWTH_HINT}`);
    }
    if (maxBytes !== undefined && bytes > maxBytes) {
      const inherited =
        baseBytes !== undefined && baseBytes > maxBytes - NEAR_CEILING_BYTES;
      (inherited ? warnings : failures).push(
        `${route}: ${bytes} Bytes überschreiten die Obergrenze ${maxBytes} Bytes in scripts/perf/bundle-ceilings.json. ${
          inherited
            ? "Die Merge-Basis lag schon an der Obergrenze; für diese PR zählt nur ihr eigener Zuwachs."
            : "Das Label gilt hier nicht: Bundle verkleinern oder die Obergrenze im PR begründet anheben."
        }`,
      );
    }
  }
  for (const route of ceilings.keys()) {
    if (!measured.has(route))
      failures.push(
        `${route}: Obergrenze ohne gemessene Route. Eintrag in scripts/perf/bundle-ceilings.json auf die neue Route umstellen oder entfernen.`,
      );
  }
  return { failures, warnings, rows };
}
