import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  MODULE_KEYS,
  MODULE_LABELS,
  MODULE_SETTING_KEYS,
  isModuleKey,
  moduleSettingKey,
  parseBookedModules,
} from "./modules";

describe("module keys", () => {
  it("mirrors the backend module list in order", () => {
    const source = readFileSync(
      resolve(
        dirname(fileURLToPath(import.meta.url)),
        "../../../backend/modules/settings/records/modules.go",
      ),
      "utf8",
    );
    const backendKeys = [
      ...source.matchAll(/^\s+Module\w+\s+ModuleKey = "([^"]+)"$/gm),
    ].map((match) => match[1]);

    expect(backendKeys).toEqual([...MODULE_KEYS]);
  });

  it("mirrors the backend module names", () => {
    const source = readFileSync(
      resolve(
        dirname(fileURLToPath(import.meta.url)),
        "../../../backend/modules/settings/records/modules.go",
      ),
      "utf8",
    );
    const backendLabels = [...source.matchAll(/Label: "([^"]+)"/g)].map(
      (match) => match[1],
    );

    expect(backendLabels).toEqual(MODULE_KEYS.map((key) => MODULE_LABELS[key]));
  });

  it("names each switch module.<key>", () => {
    expect(moduleSettingKey("zeiterfassung")).toBe("module.zeiterfassung");
    expect(MODULE_SETTING_KEYS).toHaveLength(MODULE_KEYS.length);
    expect(MODULE_SETTING_KEYS).toContain("module.nfc_anwesenheit");
  });

  it("keeps only known keys from the tenant shell", () => {
    expect(
      parseBookedModules(["dienstplan", "basis", 3, "mittagessen"]),
    ).toEqual(["dienstplan", "mittagessen"]);
    expect(isModuleKey("basis")).toBe(false);
  });

  it("books nothing for a missing or malformed list", () => {
    expect(parseBookedModules(undefined)).toEqual([]);
    expect(parseBookedModules("dienstplan")).toEqual([]);
  });
});
