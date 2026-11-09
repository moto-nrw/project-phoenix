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

  it("stehen im Code nirgends als px-Breite in einer Media Query", () => {
    const pxWidthQuery = /\((?:min|max)-width:\s*\d+(?:\.\d+)?px\)/;
    const offenders: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const path = join(dir, entry.name);
        if (entry.isDirectory()) walk(path);
        else if (
          /\.(tsx?|css)$/.test(entry.name) &&
          !/\.(test|stories)\.tsx?$/.test(entry.name) &&
          pxWidthQuery.test(readFileSync(path, "utf8"))
        ) {
          offenders.push(path);
        }
      }
    };
    walk(join(process.cwd(), "src"));

    expect(offenders).toEqual([]);
  });
});
