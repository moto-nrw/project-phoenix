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
    const offenders: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) walk(path);
        else if (
          /\.(tsx?|css)$/.test(entry.name) &&
          !/\.(test|stories)\.tsx?$/.test(entry.name)
        ) {
          const relativePath = path.slice(process.cwd().length + 1);
          offenders.push(
            ...findPxBreakpoints(readFileSync(path, "utf8")).map(
              (hit) => `${relativePath}:${hit}`,
            ),
          );
        }
      }
    };
    walk(join(process.cwd(), "src"));

    expect(offenders).toEqual([]);
  });
});

/*
 * Eine Breite in px schaltet an einer anderen Stelle als Tailwinds rem-Grenze,
 * sobald die Browserschrift nicht 16 px ist, und lässt bei Bruchteilbreiten
 * (767,5 px) eine Lücke. Gefunden werden Media Queries in jeder Schreibweise
 * (`min-width`/`max-width`, Bereichssyntax in beiden Richtungen) und
 * Vergleiche von `innerWidth` mit einer Zahl.
 */
const PX_BREAKPOINT_PATTERNS = [
  /\((?:min-|max-)?(?:device-)?width\s*:\s*\d+(?:\.\d+)?px\s*\)/,
  /\b(?:device-)?width\s*(?:<=?|>=?|=)\s*\d+(?:\.\d+)?px\b/,
  /\b\d+(?:\.\d+)?px\s*(?:<=?|>=?|=)\s*(?:device-)?width\b/,
  /\binnerWidth\s*(?:<=?|>=?)\s*\d/,
  /\d\s*(?:<=?|>=?)\s*(?:window\.|globalThis\.)?innerWidth\b/,
];

function findPxBreakpoints(source: string): string[] {
  return source
    .split("\n")
    .flatMap((line, index) =>
      PX_BREAKPOINT_PATTERNS.some((pattern) => pattern.test(line))
        ? [`${index + 1}: ${line.trim()}`]
        : [],
    );
}

describe("findPxBreakpoints", () => {
  it.each([
    "@media (width < 768px) {",
    "@media (768px <= width) {",
    'matchMedia("(767.5px >= width)")',
    "@media (min-width: 1024px) {",
    "@media screen and (max-width:767.98px) {",
    "`(min-width: 768px) and (max-width: 1279px)`",
    "setIsDesktop(window.innerWidth >= 1024);",
    "if (640 > globalThis.innerWidth) {",
  ])("findet %s", (line) => {
    expect(findPxBreakpoints(line)).toHaveLength(1);
  });

  it.each([
    "@media (width >= 64rem) {",
    "not all and (min-width: 48rem)",
    "(48rem <= width < 64rem)",
    'className="max-w-[420px] w-[calc(100%-16px)]"',
    "rect.right <= globalThis.innerWidth",
    "const viewportWidth = window.innerWidth;",
  ])("lässt %s durch", (line) => {
    // Anker: die Zeile ist nicht leer und wird geprüft.
    expect(line.length).toBeGreaterThan(0);
    expect(findPxBreakpoints(line)).toEqual([]);
  });
});
