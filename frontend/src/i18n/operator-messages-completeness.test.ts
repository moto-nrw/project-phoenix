import { describe, expect, it } from "vitest";
import { ERROR_CODES, ERROR_CODE_CLASSES } from "~/lib/error-codes.generated";
import de from "./messages/de.json";
import { OPERATOR_PORTAL_ERROR_CODES } from "~/test/operator-error-codes";

interface ErrorCatalog {
  classes: Record<string, string>;
  codes: Record<string, Record<string, string>>;
}

const german = de.errorCatalog as unknown as ErrorCatalog;

function codeText(code: string): string | undefined {
  const [area, name] = code.split(".");
  return german.codes[area!]?.[name!];
}

// Areas only the operator router answers with: a new code there must be
// added to OPERATOR_PORTAL_ERROR_CODES and get its own text.
const operatorOnlyAreas = new Set([
  "provisioning",
  "settings",
  "devices",
  "billing",
]);

describe("operator error catalog completeness (#2519)", () => {
  it("gives every operator code beyond the fallbacks its own German text", () => {
    const classTexts = new Set(Object.values(german.classes));
    const missing = OPERATOR_PORTAL_ERROR_CODES.filter((code) => {
      if (code.startsWith("general.")) return false;
      const text = codeText(code);
      return !text?.trim() || classTexts.has(text);
    });

    expect(missing).toEqual([]);
  });

  it("names each code of the operator-only areas", () => {
    const listed = new Set<string>(OPERATOR_PORTAL_ERROR_CODES);
    const unlisted = ERROR_CODES.filter(
      (code) => operatorOnlyAreas.has(code.split(".")[0]!) && !listed.has(code),
    );

    expect(unlisted).toEqual([]);
  });

  it("lists only codes the registry knows, each once", () => {
    expect(new Set(OPERATOR_PORTAL_ERROR_CODES).size).toBe(
      OPERATOR_PORTAL_ERROR_CODES.length,
    );
    for (const code of OPERATOR_PORTAL_ERROR_CODES) {
      expect(ERROR_CODE_CLASSES[code]).toBeDefined();
    }
  });
});
