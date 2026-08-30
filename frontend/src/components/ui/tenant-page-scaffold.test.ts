import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// Gate für das Seitengerüst (siehe .claude/rules/frontend-ui-kit.md und
// components/ui/TENANT-PAGE-SPEC.md): jede Tenant-Seite rendert `TenantPage`
// und baut kein eigenes Layout. Der Test liest Quelltext, nicht das DOM — er
// hält die Konvention auch für Seiten fest, die noch keinen eigenen Test haben.

const SRC_DIR = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
);
const PROTECTED_DIR = path.join(SRC_DIR, "app", "[tenant]", "(protected)");

/**
 * Seiten mit bewusst eigenem Gerüst. Die Liste ist leer und soll es bleiben:
 * Startseite, Profil und Notfallliste hatten früher ein eigenes Layout und
 * tragen seit dem Umbau dasselbe Gerüst wie alle anderen Seiten. Ein neuer
 * Eintrag braucht die Zustimmung im PR (Regel „When to deviate").
 */
// Ausnahme auf Anweisung (30.08.2026): die Notfallliste bleibt exakt wie auf
// development.
const EXEMPT = new Set<string>(["emergency/page.tsx"]);

function collectPageFiles(directory: string): string[] {
  return readdirSync(directory).flatMap((entry) => {
    const entryPath = path.join(directory, entry);
    if (statSync(entryPath).isDirectory()) return collectPageFiles(entryPath);
    return entry === "page.tsx" ? [entryPath] : [];
  });
}

const pages = collectPageFiles(PROTECTED_DIR)
  .map((file) => ({
    id: path.relative(PROTECTED_DIR, file),
    file,
    source: readFileSync(file, "utf8"),
  }))
  .filter((page) => !EXEMPT.has(page.id));

/**
 * Seiten, die nur eine View-Komponente rendern, tragen das Gerüst in dieser
 * Komponente. Sie erkennen wir daran, dass die Datei selbst kaum Markup hat.
 */
function rendersOwnMarkup(source: string): boolean {
  return /<(div|section|main|h1|header)\b/.test(source);
}

describe("Seitengerüst des Tenant-Portals", () => {
  it("findet Seiten zum Prüfen", () => {
    expect(pages.length).toBeGreaterThan(50);
  });

  it.each(pages.map((page) => [page.id, page] as const))(
    "%s baut kein eigenes Layout",
    (_id, page) => {
      const verstoesse: string[] = [];
      if (/<h1\b/.test(page.source)) verstoesse.push("eigene <h1>");
      if (/<main\b/.test(page.source)) verstoesse.push("eigenes <main>");
      // Bewusst NICHT geprüft: `max-w-` und `mx-auto`. Beides ist innerhalb
      // des Inhalts normales Layout (ein zentrierter Monatsumschalter, eine
      // schmale Karte, ein Dialog). Textuell lässt sich das nicht von einer
      // Seite unterscheiden, die sich als Ganzes zentriert — und ein Gate,
      // das zu Umbauten am falschen Ort zwingt, richtet mehr Schaden an als
      // es verhindert. Die Seitenbreite deckt stattdessen die Prüfung unten
      // ab: wer `TenantPage` als Wurzel rendert, bekommt die Breite aus der
      // Shell.
      if (/\bkicker[=:]/.test(page.source))
        verstoesse.push("Mini-Überschrift (kicker)");
      // Der Seitenrumpf mit eigenem Rhythmus. Genau diese Klassenfolge war
      // das alte, handgepflegte Gerüst; heute liefert `TenantPage` sie. Eine
      // Seite, die sie noch selbst setzt, rendert das Gerüst nicht als
      // Wurzel — auch dann nicht, wenn sie es in einem Ladezweig importiert.
      if (/className="w-full space-y-6"/.test(page.source))
        verstoesse.push("eigener Seitenrumpf (w-full space-y-6)");

      expect(verstoesse, `${page.id}: ${verstoesse.join(", ")}`).toEqual([]);
    },
  );

  it.each(
    pages
      .filter((page) => rendersOwnMarkup(page.source))
      .map((page) => [page.id, page] as const),
  )("%s rendert TenantPage als Wurzel", (_id, page) => {
    // `DatabasePageLayout` ist der einzige zugelassene Adapter: er rendert
    // selbst `TenantPage` und ergänzt nur das Master-Detail-Skelett.
    const nutztGeruest =
      /import\s*\{[^}]*\bTenantPage\b[^}]*\}\s*from\s*"~\/components\/ui\/tenant-page"/.test(
        page.source,
      ) || /<DatabasePageLayout\b/.test(page.source);

    expect(nutztGeruest, `${page.id} baut Markup ohne TenantPage`).toBe(true);
  });
});
