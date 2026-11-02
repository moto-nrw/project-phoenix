import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { ClockIcon, EnvelopeOpenIcon } from "@phosphor-icons/react/ssr";
import { MOTO_DUOTONE_TONES } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "./moto-concepts";
import { childrenConcept } from "~/lib/concepts/children";
import { dashboardConcept } from "~/lib/concepts/dashboard";
import { excusedConcept } from "~/lib/concepts/excused";
import { feedbackConcept } from "~/lib/concepts/feedback";
import { newsConcept } from "~/lib/concepts/news";
import { parentsConcept } from "~/lib/concepts/parents";
import { passkeysConcept } from "~/lib/concepts/passkeys";
import { pickupConcept } from "~/lib/concepts/pickup";
import { roomsConcept } from "~/lib/concepts/rooms";
import { schoolyardConcept } from "~/lib/concepts/schoolyard";
import { timeTrackingConcept } from "~/lib/concepts/timeTracking";
import { utilizationConcept } from "~/lib/concepts/utilization";

describe("MOTO_CONCEPTS", () => {
  it.each(["core", "status"] as const)(
    "keeps every %s concept on a unique primary color",
    (kind) => {
      const concepts = Object.values(MOTO_CONCEPTS).filter(
        (concept) => concept.kind === kind,
      );
      const colors = concepts.map(
        (concept) => MOTO_DUOTONE_TONES[concept.tone].primary,
      );

      expect(new Set(colors).size).toBe(colors.length);
    },
  );

  it("keeps every concept visually distinct via icon and tone", () => {
    const renderings = Object.values(MOTO_CONCEPTS).map((concept) =>
      [concept.icon.displayName, concept.tone].join("/"),
    );

    expect(new Set(renderings).size).toBe(renderings.length);
  });

  it("assigns distinct semantic tones to dashboard concepts", () => {
    expect(dashboardConcept.tone).toBe("neutral");
    expect(childrenConcept.tone).toBe("greenVivid");
    expect(roomsConcept.tone).toBe("navy");
    expect(parentsConcept.tone).toBe("blue");
    expect(timeTrackingConcept.tone).toBe("timeTracking");
    expect(feedbackConcept.tone).toBe("coral");
    expect(excusedConcept.tone).toBe("purple");
    expect(schoolyardConcept.tone).toBe("orange");
    expect(utilizationConcept.tone).toBe("gold");
  });

  it("renders passkeys as a clean outline without a duotone fill", () => {
    expect(passkeysConcept.weight).toBe("regular");
  });

  it("represents pickup changes as a time change", () => {
    expect(pickupConcept.icon).toBe(ClockIcon);
    expect(pickupConcept.tone).toBe("gold");
  });

  it("represents parent letters as received post", () => {
    expect(newsConcept.label).toBe("Elternbriefe");
    expect(newsConcept.icon).toBe(EnvelopeOpenIcon);
    expect(newsConcept.tone).toBe("blue");
  });
});

// Ein Konzept liegt je in einem eigenen Modul, damit eine Seite nur die
// Glyphen lädt, die sie zeigt. Die Registry trägt alle Glyphen (rund 73 kB);
// ein einziger Laufzeit-Import aus Produktionscode brächte sie in jede Route,
// die diesen Code lädt (#5135).
const SRC_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), "..");
const CONCEPTS_DIR = path.join(SRC_DIR, "lib", "concepts");

function productionSources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const file = path.join(dir, entry.name);
    if (entry.isDirectory()) return productionSources(file);
    if (!/\.tsx?$/.test(entry.name)) return [];
    if (/\.(test|stories)\.tsx?$|\.d\.ts$/.test(entry.name)) return [];
    return [file];
  });
}

// Alias or relative path; import, re-export and dynamic import(). Only
// `import type` / `export type` stays out of the bundle.
const RUNTIME_REGISTRY_IMPORT =
  /^\s*(?:import|export)\s+(?!type\b)[^;]*from\s+["'][^"']*\/moto-concepts["']|\bimport\(\s*["'][^"']*\/moto-concepts["']\s*\)/m;

describe("concept modules", () => {
  it("keeps every definition under the key of its registry entry", () => {
    for (const [key, concept] of Object.entries(MOTO_CONCEPTS)) {
      expect(concept.key).toBe(key);
    }
  });

  it("gives every concept its own module named after its key", () => {
    const modules = readdirSync(CONCEPTS_DIR)
      .filter((name) => name !== "define.ts")
      .map((name) => name.replace(/\.ts$/, ""))
      .sort();

    expect(modules).toEqual(Object.keys(MOTO_CONCEPTS).sort());
    for (const key of modules) {
      const source = readFileSync(path.join(CONCEPTS_DIR, `${key}.ts`), "utf8");
      expect(source).toContain(
        `export const ${key}Concept = defineMotoConcept(`,
      );
      expect(source.match(/from "@phosphor-icons\/react\/ssr"/g)).toHaveLength(
        1,
      );
    }
  });

  it("never loads the whole registry at runtime in production code", () => {
    const offenders = productionSources(SRC_DIR)
      .filter((file) => !file.endsWith(path.join("lib", "moto-concepts.ts")))
      .filter((file) =>
        RUNTIME_REGISTRY_IMPORT.test(readFileSync(file, "utf8")),
      )
      .map((file) => path.relative(SRC_DIR, file));

    expect(offenders).toEqual([]);
  });
});
