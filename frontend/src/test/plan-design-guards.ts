import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";

/**
 * Shared patterns of the planning design guards. Each guarded folder keeps
 * its own guard test next to its files (#6582): `ui/planning/` for the kit
 * primitives, `timetable/` and `staff/` for the screens built from them.
 *
 *   (a) No gradients (`bg-gradient`, `from-[`, `to-[`), except the single
 *       sanctioned `repeating-linear-gradient` hatch that is hard-encapsulated
 *       in `ui/planning/plan-block.tsx` and nowhere else.
 *   (b) No generic Tailwind bright-color utility classes
 *       ((text|bg|border)-{color}-{shade}) used for semantics. `gray-*` is
 *       exempt — semantic color always routes through LOCATION_COLORS hex
 *       or a color prop, per .claude/rules/frontend-ui-kit.md.
 */
export const GRADIENT_PATTERN = /bg-gradient|from-\[|to-\[/g;
export const BRIGHT_COLOR_PATTERN =
  /(text|bg|border)-(red|green|blue|sky|orange|purple|amber|yellow|emerald|pink|rose|indigo|violet|cyan|teal|lime|fuchsia)-\d+/g;

/** Marker of the hatch; only `plan-block.tsx` may contain it. */
export const HATCH_MARKER = "repeating-linear-gradient";

export function countMatches(source: string, pattern: RegExp): number {
  return [...source.matchAll(pattern)].length;
}

export function readSource(dir: string, file: string): string {
  return readFileSync(path.join(dir, file), "utf-8");
}

/** Recursively lists .tsx files under `dir`, returning paths relative to `dir` (posix-style). */
export function listTsxFiles(dir: string, relativeTo: string = dir): string[] {
  const entries = readdirSync(dir);
  const results: string[] = [];
  for (const name of entries) {
    const fullPath = path.join(dir, name);
    if (statSync(fullPath).isDirectory()) {
      results.push(...listTsxFiles(fullPath, relativeTo));
    } else if (name.endsWith(".tsx")) {
      results.push(
        path.relative(relativeTo, fullPath).split(path.sep).join("/"),
      );
    }
  }
  return results.sort();
}

// Only production source files are checked for the hatch — .test.tsx and
// .stories.tsx siblings legitimately reference the same literal to verify
// it, which isn't a second implementation of the hatch, just a test
// asserting on the one in plan-block.tsx.
export function isProductionSourceFile(file: string): boolean {
  return !file.endsWith(".test.tsx") && !file.endsWith(".stories.tsx");
}
