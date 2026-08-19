import { describe, expect, it } from "vitest";
import de from "./messages/de.json";
import ru from "./messages/ru.json";
import sq from "./messages/sq.json";

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
  ])("does not leave German master-data copy in %s", (_locale, catalog) => {
    const german = flatten(de.parentMasterData as MessageTree);
    const translated = flatten(catalog as MessageTree);
    const untranslated = Object.entries(translated).filter(
      ([key, value]) => value === german[key] && !invariantValues.has(value),
    );

    expect(untranslated).toEqual([]);
  });
});
