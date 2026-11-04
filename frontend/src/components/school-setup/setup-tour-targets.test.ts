import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import {
  SETUP_TOUR_BRANCHES,
  SETUP_TOURS,
  type SetupTour,
} from "./setup-tours";
import { staffOnboardingTours } from "../staff-onboarding/staff-onboarding-steps";

// Jede Stelle, auf die eine Tour per `data-setup-tour` zeigt, muss eine Seite
// auch rendern (#5142: Der Unit-Test der Tour zeigte auf eine nachgebaute
// Stelle und blieb grün, während die App „nicht zu sehen“ meldete). Und jede
// Datei, die eine solche Stelle rendert, steht in den Pfaden, bei denen
// .github/workflows/e2e.yml die Tour-Spec e2e/onboarding-tours.spec.ts auf dem
// PR startet.

const FRONTEND = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../..",
);
const REPO = path.resolve(FRONTEND, "..");
const SRC = path.join(FRONTEND, "src");

/** Die Touren selbst nennen die Stellen nur, sie rendern sie nicht. */
const TOUR_DEFINITIONS = new Set([
  "src/components/school-setup/setup-tours.ts",
  "src/components/staff-onboarding/staff-onboarding-steps.ts",
]);

function sourceFiles(directory: string): string[] {
  return readdirSync(directory).flatMap((entry) => {
    const full = path.join(directory, entry);
    if (statSync(full).isDirectory()) {
      return entry === "test" || entry === "__tests__" ? [] : sourceFiles(full);
    }
    return /\.(ts|tsx)$/.test(entry) && !/\.(test|spec|stories)\./.test(entry)
      ? [full]
      : [];
  });
}

const sources = sourceFiles(SRC)
  .map((file) => ({
    file: path.relative(FRONTEND, file).split(path.sep).join("/"),
    text: readFileSync(file, "utf8"),
  }))
  .filter(({ file }) => !TOUR_DEFINITIONS.has(file));

/** Der Ausdruck nach `name=` oder `name:` bis zu seinem Ende. */
function expressionAfter(text: string, start: number): string {
  const open = text[start];
  if (open === '"' || open === "'") {
    const end = text.indexOf(open, start + 1);
    return text.slice(start, end + 1);
  }
  if (open !== "{") {
    // `tourTarget: "…"` in einem Objekt: bis zum Ende der Zeile.
    const end = text.indexOf("\n", start);
    return text.slice(start, end === -1 ? undefined : end);
  }
  let depth = 0;
  for (let index = start; index < text.length; index++) {
    if (text[index] === "{") depth++;
    if (text[index] === "}") depth--;
    if (depth === 0) return text.slice(start, index + 1);
  }
  return text.slice(start);
}

interface RenderedTarget {
  file: string;
  /** Ein fester Wert oder ein Muster aus einem Template-String. */
  value: string | RegExp;
}

/**
 * Was eine Datei als Tour-Stelle rendert: `data-setup-tour`, die Props
 * `tourId` (Seitenleiste) und `tourTarget` (OverflowMenu-Eintrag).
 */
function renderedTargets(): { targets: RenderedTarget[]; files: string[] } {
  const targets: RenderedTarget[] = [];
  const files = new Set<string>();
  const attribute = /(?:data-setup-tour|tourId|tourTarget)\s*[=:]\s*/g;
  for (const { file, text } of sources) {
    for (const match of text.matchAll(attribute)) {
      const expression = expressionAfter(text, match.index + match[0].length);
      // Nur Werte und Ausdrücke, kein Vorgabewert wie `tourTarget = false`.
      if (!/^["'{]/.test(expression)) continue;
      // Dieselben Props tragen in der Elterntour `data-parent-tour`.
      const prop = !match[0].startsWith("data-setup-tour");
      if (prop && text.includes("data-parent-tour")) continue;
      files.add(file);
      for (const literal of expression.matchAll(/"([^"]+)"|'([^']+)'/g)) {
        targets.push({ file, value: literal[1] ?? literal[2]! });
      }
      for (const template of expression.matchAll(/`([^`]+)`/g)) {
        const pattern = template[1]!
          .split(/\$\{[^}]+\}/)
          .map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
          .join("(.+)");
        targets.push({ file, value: new RegExp(`^${pattern}$`) });
      }
    }
  }
  return { targets, files: [...files].sort() };
}

/** Alle `data-setup-tour`-Werte, die eine Tour sucht. */
function tourTargetValues(): Map<string, string> {
  const tours: Record<string, SetupTour> = {};
  for (const [key, tour] of Object.entries(SETUP_TOURS)) {
    tours[`Einrichtung ${key}`] = tour;
  }
  for (const [key, tour] of Object.entries(SETUP_TOUR_BRANCHES)) {
    tours[`Zweig ${key}`] = tour;
  }
  for (const childSearchOnly of [false, true]) {
    for (const [key, tour] of Object.entries(
      staffOnboardingTours(childSearchOnly),
    )) {
      tours[
        `Erste Schritte ${key}${childSearchOnly ? " (nur Alle Kinder)" : ""}`
      ] = tour;
    }
  }
  const values = new Map<string, string>();
  for (const [name, tour] of Object.entries(tours)) {
    for (const stop of tour.stops) {
      const selectors = [
        stop.target,
        stop.skipWhenVisible,
        stop.endBefore,
        stop.advanceWhenVisible,
        stop.textWhenVisible?.selector,
      ];
      for (const selector of selectors) {
        for (const match of (selector ?? "").matchAll(
          /data-setup-tour="([^"]+)"/g,
        )) {
          values.set(match[1]!, `${name}, Station „${stop.title}“`);
        }
      }
    }
  }
  return values;
}

/**
 * Ein Muster wie `nav-${item.href}` zählt nur, wenn sein variabler Teil
 * (`/students/search`) irgendwo im Quelltext als String steht, etwa als
 * `href` der Seitenleiste.
 */
function isRendered(value: string, targets: readonly RenderedTarget[]) {
  return targets.some((target) => {
    if (typeof target.value === "string") return target.value === value;
    const match = target.value.exec(value);
    if (!match) return false;
    return match
      .slice(1)
      .every((part) =>
        sources.some(
          ({ text }) =>
            text.includes(`"${part}"`) || text.includes(`'${part}'`),
        ),
      );
  });
}

/** `paths` unter `pull_request` in .github/workflows/e2e.yml. */
function e2ePullRequestPaths(): string[] {
  const workflow = readFileSync(
    path.join(REPO, ".github/workflows/e2e.yml"),
    "utf8",
  );
  const block =
    /\n {2}pull_request:\n[\s\S]*?\n {4}paths:\n((?: {6}(?:- |#).*\n)+)/.exec(
      workflow,
    );
  if (!block) throw new Error("e2e.yml hat keine pull_request.paths.");
  return block[1]!
    .split("\n")
    .map((line) => /^ {6}- "?([^"]+)"?$/.exec(line)?.[1])
    .filter((entry): entry is string => entry !== undefined);
}

/** GitHub-Pfadfilter: `**` über Ordner, `*` innerhalb eines Ordners. */
function globToRegExp(glob: string): RegExp {
  let pattern = "";
  for (let index = 0; index < glob.length; index++) {
    const char = glob[index]!;
    if (glob.startsWith("**/", index)) {
      pattern += "(?:.*/)?";
      index += 2;
    } else if (glob.startsWith("**", index)) {
      pattern += ".*";
      index += 1;
    } else if (char === "*") {
      pattern += "[^/]*";
    } else {
      pattern += char.replace(/[.+?^${}()|[\]\\]/g, "\\$&");
    }
  }
  return new RegExp(`^${pattern}$`);
}

function triggersE2E(file: string, paths: readonly string[]): boolean {
  let included = false;
  for (const entry of paths) {
    const negated = entry.startsWith("!");
    if (globToRegExp(negated ? entry.slice(1) : entry).test(file)) {
      included = !negated;
    }
  }
  return included;
}

describe("Stellen der geführten Touren", () => {
  const { targets, files } = renderedTargets();

  it("findet die Stellen und Touren überhaupt", () => {
    // Anker: Ohne Fund prüften die Tests unten nichts.
    expect(files).toContain("src/components/admin/invitation-form.tsx");
    expect(tourTargetValues().get("invite-submit")).toContain(
      "Einrichtung team",
    );
  });

  it("jede gesuchte data-setup-tour-Stelle rendert eine Seite", () => {
    const missing = [...tourTargetValues()]
      .filter(([value]) => !isRendered(value, targets))
      .map(([value, where]) => `data-setup-tour="${value}" (${where})`);
    expect(missing).toEqual([]);
  });

  it("jede Datei mit einer Tour-Stelle startet die Tour-Spec in e2e.yml", () => {
    const paths = e2ePullRequestPaths();
    const repoFiles = [
      ...files,
      ...TOUR_DEFINITIONS,
      "src/components/school-setup/use-setup-tour.ts",
      "src/components/ui/coach-mark.tsx",
      "e2e/onboarding-tours.spec.ts",
    ].map((file) => `frontend/${file}`);
    expect(repoFiles.filter((file) => !triggersE2E(file, paths))).toEqual([]);
  });

  it("prüft die Pfadmuster wie GitHub", () => {
    expect(
      triggersE2E("frontend/src/app/[tenant]/(protected)/rooms/page.tsx", [
        "frontend/src/app/**/rooms/page.tsx",
      ]),
    ).toBe(true);
    expect(
      triggersE2E("frontend/e2e/layout/a.spec.ts", [
        "frontend/e2e/**",
        "!frontend/e2e/layout/**",
      ]),
    ).toBe(false);
    expect(
      triggersE2E("frontend/src/components/a/b.tsx", [
        "frontend/src/components/*.tsx",
      ]),
    ).toBe(false);
  });
});
