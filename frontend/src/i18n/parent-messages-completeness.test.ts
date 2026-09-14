import { describe, expect, it } from "vitest";
import de from "./messages/de.json";
import pl from "./messages/pl.json";
import ru from "./messages/ru.json";
import sq from "./messages/sq.json";
import tr from "./messages/tr.json";
import uk from "./messages/uk.json";

interface MessageTree {
  [key: string]: string | MessageTree;
}

function flatten(tree: MessageTree, prefix = ""): Record<string, string> {
  return Object.fromEntries(
    Object.entries(tree).flatMap(([key, value]) => {
      const path = prefix ? `${prefix}.${key}` : key;
      return typeof value === "string"
        ? [[path, value]]
        : Object.entries(flatten(value, path));
    }),
  );
}

const invariantValues = new Set(["Bus", "E-Mail", "SMS", "Status", "Telefon"]);

describe("parent message catalog completeness", () => {
  it.each([
    ["ru", ru.parentMasterData],
    ["sq", sq.parentMasterData],
    ["pl", pl.parentMasterData],
    ["tr", tr.parentMasterData],
    ["uk", uk.parentMasterData],
  ])("does not leave German master-data copy in %s", (_locale, catalog) => {
    const german = flatten(de.parentMasterData as MessageTree);
    const translated = flatten(catalog as MessageTree);
    const untranslated = Object.entries(translated).filter(
      ([key, value]) => value === german[key] && !invariantValues.has(value),
    );

    expect(untranslated).toEqual([]);
  });
});

// pl, tr and uk were translated as complete catalogs (#3202). Unlike ru and sq
// they carry no German leftovers anywhere, so the whole catalog is guarded: a
// value equal to German would reach families as German text without warning.
const fullCatalogInvariants = new Set(["OGS", "OGS: {text}", "PDF", "Excel"]);

describe("complete catalogs for pl, tr and uk", () => {
  it.each([
    ["pl", pl],
    ["tr", tr],
    ["uk", uk],
  ])("has no German copy left in %s", (_locale, catalog) => {
    const german = flatten(de as unknown as MessageTree);
    const translated = flatten(catalog as unknown as MessageTree);
    const untranslated = Object.entries(translated).filter(
      ([key, value]) =>
        value === german[key] &&
        !invariantValues.has(value) &&
        !fullCatalogInvariants.has(value),
    );

    expect(untranslated).toEqual([]);
  });
});
