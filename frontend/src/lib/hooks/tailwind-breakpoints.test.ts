import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { TAILWIND_BREAKPOINTS } from "./use-media-query";

describe("Tailwind-Breakpoints", () => {
  it("stimmen mit dem Theme überein, aus dem Tailwind die Varianten baut", () => {
    const themeCss = readFileSync(
      join(process.cwd(), "node_modules/tailwindcss/theme.css"),
      "utf8",
    );
    const globalsCss = readFileSync(
      join(process.cwd(), "src/styles/globals.css"),
      "utf8",
    );
    const breakpoint = /--breakpoint-([a-z0-9]+):\s*([^;]+);/g;
    const theme = new Map<string, string>();
    // globals.css kommt nach dem Tailwind-Theme und würde es überschreiben.
    for (const css of [themeCss, globalsCss]) {
      for (const [, name, value] of css.matchAll(breakpoint)) {
        theme.set(name!, value!.trim());
      }
    }

    for (const [name, value] of Object.entries(TAILWIND_BREAKPOINTS)) {
      expect(theme.get(name), `--breakpoint-${name}`).toBe(value);
    }
  });

  it("stehen im Code nirgends als px-Breite", () => {
    const hits: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) walk(path);
        else if (
          /\.(tsx?|css)$/.test(entry.name) &&
          !/\.(test|stories)\.tsx?$/.test(entry.name)
        ) {
          const relativePath = path.slice(process.cwd().length + 1);
          hits.push(
            ...findPxBreakpoints(readFileSync(path, "utf8")).map(
              (hit) => `${relativePath}:${hit}`,
            ),
          );
        }
      }
    };
    walk(join(process.cwd(), "src"));

    const allowed = (hit: string) =>
      PX_BREAKPOINT_ALLOWLIST.some(
        (entry) =>
          hit.startsWith(`${entry.file}:`) && hit.includes(entry.match),
      );
    expect(hits.filter((hit) => !allowed(hit))).toEqual([]);
    // Eine Ausnahme, deren Stelle es nicht mehr gibt, fällt heraus.
    for (const entry of PX_BREAKPOINT_ALLOWLIST) {
      expect(
        hits.some(
          (hit) =>
            hit.startsWith(`${entry.file}:`) && hit.includes(entry.match),
        ),
        `${entry.file}: ${entry.match}`,
      ).toBe(true);
    }
  });

  it("hat eine Allowlist, die nur schrumpft", () => {
    // Anker: die Obergrenze ist eine Zahl.
    expect(PX_BREAKPOINT_ALLOWLIST_CEILING).toBeGreaterThanOrEqual(0);
    expect(PX_BREAKPOINT_ALLOWLIST.length).toBeLessThanOrEqual(
      PX_BREAKPOINT_ALLOWLIST_CEILING,
    );
  });
});

/*
 * Invariante: Der Guard schlägt bei jedem Vergleich einer Breitenquelle mit
 * einer Zahl an, in beiden Richtungen (`innerWidth < 768`, `768 > innerWidth`),
 * und bei jeder px-Breite in einer Media Query (`min-width`/`max-width`,
 * Bereichssyntax in beiden Richtungen). Eine Breite in px schaltet an einer
 * anderen Stelle als Tailwinds rem-Grenze, sobald die Browserschrift nicht
 * 16 px ist, und lässt bei Bruchteilbreiten (767,5 px) eine Lücke. Abfragen
 * gehören über `below`/`atLeast`/`between` aus `use-media-query.ts`.
 *
 * Gesucht wird im ganzen Quelltext, nicht zeilenweise: ein Umbruch zwischen
 * Quelle, Operator und Zahl hilft nicht.
 */
const WIDTH_SOURCES = [
  "innerWidth",
  "outerWidth",
  "screen.width",
  "screen.availWidth",
  "visualViewport.width",
  "document.documentElement.clientWidth",
  "document.body.clientWidth",
] as const;

/** Ausnahmen mit Begründung: `file` relativ zu `frontend/`, `match` aus der Fundzeile. */
const PX_BREAKPOINT_ALLOWLIST: ReadonlyArray<{
  readonly file: string;
  readonly match: string;
  readonly reason: string;
}> = [];
/** Darf nur sinken; eine neue Ausnahme braucht eine Begründung im Review. */
const PX_BREAKPOINT_ALLOWLIST_CEILING = 0;

const escapeRegExp = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
// `window.` oder `globalThis.` davor ist optional, `?.` zählt wie `.`.
const WIDTH_SOURCE = `(?<![\\w$.])(?:(?:window|globalThis|self)\\??\\.)?(?:${WIDTH_SOURCES.map(
  (source) => escapeRegExp(source).replaceAll("\\.", "\\??\\."),
).join("|")})(?![\\w$])`;
const COMPARISON = "(?:===?|!==?|<=?|>=?)";
const NUMBER = "-?\\d+(?:\\.\\d+)?(?![\\w$])";
const NUMBER_LEFT = "(?<![\\w$.])-?\\d+(?:\\.\\d+)?";
const PX = "\\d+(?:\\.\\d+)?px\\b";

const PX_BREAKPOINT_PATTERNS = [
  new RegExp(`${WIDTH_SOURCE}\\s*${COMPARISON}\\s*${NUMBER}`, "g"),
  new RegExp(`${NUMBER_LEFT}\\s*${COMPARISON}\\s*${WIDTH_SOURCE}`, "g"),
  new RegExp(`\\((?:min-|max-)?(?:device-)?width\\s*:\\s*${PX}\\s*\\)`, "g"),
  new RegExp(`\\b(?:device-)?width\\s*(?:<=?|>=?|=)\\s*${PX}`, "g"),
  new RegExp(`${PX}\\s*(?:<=?|>=?|=)\\s*(?:device-)?width\\b`, "g"),
];

function findPxBreakpoints(source: string): string[] {
  const lines = source.split("\n");
  const hitLines = new Set<number>();
  for (const pattern of PX_BREAKPOINT_PATTERNS) {
    for (const match of source.matchAll(pattern)) {
      hitLines.add(source.slice(0, match.index).split("\n").length);
    }
  }
  return [...hitLines]
    .sort((a, b) => a - b)
    .map((line) => `${line}: ${lines[line - 1]!.trim()}`);
}

describe("findPxBreakpoints", () => {
  const prefixes = ["", "window.", "globalThis."];
  const sourceCases = WIDTH_SOURCES.flatMap((source) =>
    prefixes.flatMap((prefix) => [
      `if (${prefix}${source} < 768) {`,
      `if (768 > ${prefix}${source}) {`,
    ]),
  );

  it.each([
    ...sourceCases,
    "setIsDesktop(window.innerWidth >= 1024);",
    "const wide = window.visualViewport?.width >= 1280;",
    "return 640 <= document.documentElement?.clientWidth;",
    "if (window.screen.width === 390) {",
    "if (\n  window.outerWidth <\n  768\n) {",
    "@media (width < 768px) {",
    "@media (768px <= width) {",
    'matchMedia("(767.5px >= width)")',
    "@media (min-width: 1024px) {",
    "@media screen and (max-width:767.98px) {",
    "`(min-width: 768px) and (max-width: 1279px)`",
  ])("findet %s", (code) => {
    expect(findPxBreakpoints(code)).toHaveLength(1);
  });

  it.each([
    "@media (width >= 64rem) {",
    "not all and (min-width: 48rem)",
    "(48rem <= width < 64rem)",
    'className="max-w-[420px] w-[calc(100%-16px)]"',
    "rect.right <= globalThis.innerWidth",
    "const viewportWidth = window.innerWidth;",
    "const left = window.innerWidth - rect.right - 8;",
    "Math.min(window.screen.width, 640)",
    "if (rect.width < 640) {",
    "if (panel.clientWidth < 320) {",
    "if (window.innerHeight < 700) {",
  ])("lässt %s durch", (code) => {
    // Anker: der Ausschnitt ist nicht leer und wird geprüft.
    expect(code.length).toBeGreaterThan(0);
    expect(findPxBreakpoints(code)).toEqual([]);
  });
});
