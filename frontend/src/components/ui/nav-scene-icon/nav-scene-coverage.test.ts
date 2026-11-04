import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { NAV_SCENES } from "./nav-scenes";

// Gate für die Icon-Szenen der Seitenleisten (README.md daneben): Jedes
// Glyph, das eine Seitenleiste zeigt, braucht eine Szene, und jede Szene
// bewegt Teile des Icons. Der Test liest Quelltext, damit eine neue Zeile in
// irgendeiner Seitenleiste ohne eigenen Test trotzdem auffällt.

const SRC_DIR = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
);
const SCENES_CSS = readFileSync(
  path.join(SRC_DIR, "styles", "nav-scenes.css"),
  "utf8",
);

/** Dateien, aus denen die Seitenleisten ihre Einträge und Icons beziehen. */
function sidebarSourceFiles(): string[] {
  const out: string[] = [path.join(SRC_DIR, "lib", "staff-navigation.ts")];
  const walk = (directory: string) => {
    for (const entry of readdirSync(directory)) {
      const entryPath = path.join(directory, entry);
      if (statSync(entryPath).isDirectory()) walk(entryPath);
      else if (
        /(sidebar[\w-]*|-nav-items)\.tsx?$/.test(entry) &&
        !/\.(test|stories)\.tsx?$/.test(entry)
      ) {
        out.push(entryPath);
      }
    }
  };
  walk(path.join(SRC_DIR, "components"));
  return out;
}

const conceptGlyphs = new Map<string, string>(
  Object.values(MOTO_CONCEPTS).map((concept) => [
    concept.key,
    concept.icon.displayName ?? "",
  ]),
);

/** Glyphen je Datei: über Konzepte und direkt aus Phosphor importierte. */
function glyphsUsedBy(source: string): { glyph: string; via: string }[] {
  const used: { glyph: string; via: string }[] = [];
  for (const [, key] of source.matchAll(
    /from\s+["']~\/lib\/concepts\/(\w+)["']/g,
  )) {
    const concept = Object.values(MOTO_CONCEPTS).find(
      (entry) => entry.key === key,
    );
    const glyph = concept ? conceptGlyphs.get(concept.key) : undefined;
    if (glyph) used.push({ glyph, via: `Konzept ${key}` });
  }
  for (const [, names] of source.matchAll(
    /import\s*{([^}]*)}\s*from\s*["']@phosphor-icons\/react(?:\/[\w-]+)?["']/g,
  )) {
    for (const name of (names ?? "").split(",").map((part) => part.trim())) {
      if (/^[A-Z]\w*Icon$/.test(name)) {
        used.push({ glyph: name, via: "Phosphor-Import" });
      }
    }
  }
  return used;
}

/**
 * Klassen mit eigener Bewegung je Szene:
 * `.ns-play[data-nav-scene="ClockIcon"] .ns-min{animation:…}`.
 */
const animatedClasses = new Map<string, Set<string>>();
for (const [, glyph = "", name = ""] of SCENES_CSS.matchAll(
  /\.ns-play\[data-nav-scene="(\w+)"\]\s+\.(ns-[\w-]+)\s*{\s*animation:/g,
)) {
  animatedClasses.set(
    glyph,
    (animatedClasses.get(glyph) ?? new Set()).add(name),
  );
}
const definedKeyframes = new Set(
  [...SCENES_CSS.matchAll(/@keyframes\s+(ns-[\w-]+)/g)].map(
    (match) => match[1],
  ),
);

describe("Icon-Szenen der Seitenleisten", () => {
  it("hat für jedes Glyph einer Seitenleiste eine Szene", () => {
    const missing = sidebarSourceFiles().flatMap((file) =>
      glyphsUsedBy(readFileSync(file, "utf8"))
        .filter(({ glyph }) => !NAV_SCENES[glyph])
        .map(
          ({ glyph, via }) =>
            `${glyph} (${via}, ${path.relative(SRC_DIR, file)})`,
        ),
    );
    expect(
      [...new Set(missing)],
      "Seitenleisten-Icon ohne Szene. Lege sie in nav-scenes.tsx und " +
        "styles/nav-scenes.css an, Vorgehen in nav-scene-icon/README.md.",
    ).toEqual([]);
  });

  it.each(Object.entries(NAV_SCENES))(
    "%s bewegt Teile, jedes Element mit höchstens einer Bewegung",
    (glyph, scene) => {
      expect(scene.description.trim()).not.toBe("");
      const markup = renderToStaticMarkup(
        createElement("svg", null, scene.render("test")),
      );
      const classes = new Set(
        [...markup.matchAll(/class="([^"]+)"/g)].flatMap((match) =>
          (match[1] ?? "").split(/\s+/),
        ),
      );
      const moving = [...classes].filter((name) =>
        animatedClasses.get(glyph)?.has(name),
      );
      expect(moving, `${glyph}: kein Teil mit Animation`).not.toEqual([]);

      // Zwei Bewegungen auf einem Element: Die spätere Regel ersetzt die
      // ganze `animation`, die frühere fällt still weg. Teile, die sich
      // gemeinsam bewegen, gehören in ein `<g>` mit der äußeren Bewegung.
      const doubled = [...markup.matchAll(/class="([^"]+)"/g)]
        .map((match) =>
          (match[1] ?? "")
            .split(/\s+/)
            .filter((name) => animatedClasses.get(glyph)?.has(name)),
        )
        .filter((names) => names.length > 1)
        .map((names) => names.join(" "));
      expect(doubled, `${glyph}: zwei Bewegungen auf einem Element`).toEqual(
        [],
      );
    },
  );

  it("bindet jede Teil-Regel an ihre Szene", () => {
    // Gleiche Teilnamen (`ns-draw`, `ns-dot`) kommen in mehreren Szenen vor.
    // Ohne Glyph im Selektor gewinnt die letzte Regel in allen Szenen.
    expect(
      [...SCENES_CSS.matchAll(/(?:^|[\s,}])(\.ns-play\s+\.ns-[\w-]+)/g)].map(
        (match) => match[1],
      ),
    ).toEqual([]);
    const scoped = new Set(
      [...SCENES_CSS.matchAll(/\[data-nav-scene="(\w+)"\]/g)].map(
        (match) => match[1] ?? "",
      ),
    );
    expect([...scoped].filter((glyph) => !NAV_SCENES[glyph])).toEqual([]);
  });

  it("verweist nur auf definierte Keyframes", () => {
    const used = [...SCENES_CSS.matchAll(/animation:\s*(ns-[\w-]+)/g)].map(
      (match) => match[1],
    );
    expect(used.filter((name) => !definedKeyframes.has(name))).toEqual([]);
  });

  it("schaltet alle Bewegung bei reduzierter Bewegung ab", () => {
    expect(SCENES_CSS).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)\s*{\s*\.ns-play,\s*\.ns-play \*\s*{\s*animation:\s*none\s*!important/,
    );
  });
});
