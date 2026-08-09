import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

/**
 * The brand palette exists twice: as `--color-moto-*` custom properties in
 * globals.css (which every `bg-moto-*` / `text-moto-*` utility compiles from)
 * and as MOTO_COLOR_PALETTE in location-helper.ts (which every inline style
 * and every LOCATION_BADGE_TONES entry reads). Nothing forced the two to
 * agree.
 *
 * That is not hypothetical: a component like RoomStatusBadge draws its pill
 * from a utility class and its dot from the TS palette, so a value changed in
 * only one copy paints the same element in two different reds. The Go drift
 * test guards the backend's reserved-colour list against LOCATION_COLORS; this
 * is the missing third edge of that triangle.
 *
 * Naming contract: `--color-moto-<family>` is the family's `base`, and
 * `--color-moto-<family>-<step>` is that step. Families and steps that exist
 * on only one side are ignored on purpose — the CSS carries a few tokens with
 * no TS counterpart (and vice versa); what must never differ is a value
 * present in both.
 */

const CSS_VAR = /^\s*--color-moto-([a-z0-9-]+):\s*(#[0-9a-fA-F]{6})\s*;/gm;

function readCssTokens(): Map<string, string> {
  const css = readFileSync(
    join(process.cwd(), "src/styles/globals.css"),
    "utf8",
  );
  const tokens = new Map<string, string>();
  for (const match of css.matchAll(CSS_VAR)) {
    tokens.set(match[1]!, match[2]!.toLowerCase());
  }
  return tokens;
}

/** camelCase palette family → the kebab-case slug used in the CSS token. */
function cssFamily(family: string): string {
  return family.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`);
}

describe("moto palette tokens", () => {
  const cssTokens = readCssTokens();

  it("finds the token block at all", () => {
    // Guards the regex itself: a restructured globals.css must fail loudly
    // here rather than silently turning this whole suite into a no-op.
    expect(cssTokens.size).toBeGreaterThan(50);
  });

  it("keeps every shared value identical to MOTO_COLOR_PALETTE", () => {
    const mismatches: string[] = [];

    for (const [family, steps] of Object.entries(MOTO_COLOR_PALETTE)) {
      for (const [step, hex] of Object.entries(steps)) {
        const slug =
          step === "base"
            ? cssFamily(family)
            : `${cssFamily(family)}-${step.toLowerCase()}`;
        const cssHex = cssTokens.get(slug);
        if (cssHex === undefined) continue;
        if (cssHex !== hex.toLowerCase()) {
          mismatches.push(
            `--color-moto-${slug}: ${cssHex} !== MOTO_COLOR_PALETTE.${family}.${step} (${hex.toLowerCase()})`,
          );
        }
      }
    }

    expect(mismatches).toEqual([]);
  });
});
