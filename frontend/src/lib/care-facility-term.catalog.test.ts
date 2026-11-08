import { describe, expect, it, vi } from "vitest";
import de from "~/i18n/messages/de.generated";
import en from "~/i18n/messages/en.generated";
import pl from "~/i18n/messages/pl.generated";
import ru from "~/i18n/messages/ru.generated";
import sq from "~/i18n/messages/sq.generated";
import tr from "~/i18n/messages/tr.generated";
import uk from "~/i18n/messages/uk.generated";

// The enrollment flow names the school's care through `{careTerm}` in every
// language (#6042): no fixed „OGS“ is left, and every language carries the
// placeholder wherever German does, so switching the language keeps „GTS“.
const LOCALES = { de, en, pl, ru, sq, tr, uk } as const;
const NAMESPACES = [
  "enrollmentPublic",
  "enrollmentForm",
  "enrollmentStatus",
] as const;

function leaves(node: unknown, prefix = ""): [string, string][] {
  if (typeof node === "string") return [[prefix, node]];
  if (!node || typeof node !== "object") return [];
  return Object.entries(node).flatMap(([key, value]) =>
    leaves(value, prefix ? `${prefix}.${key}` : key),
  );
}

describe("care facility term in the enrollment catalogs", () => {
  const germanKeys = NAMESPACES.flatMap((namespace) =>
    leaves(de[namespace], namespace)
      .filter(([, text]) => text.includes("{careTerm}"))
      .map(([key]) => key),
  );

  it("covers the texts that used to name the OGS", () => {
    expect(germanKeys).toContain("enrollmentPublic.reviewText");
    expect(germanKeys).toContain("enrollmentStatus.stepOneStatus");
    expect(germanKeys.length).toBeGreaterThanOrEqual(33);
  });

  it.each(Object.keys(LOCALES))(
    "%s formats every enrollment text with the school's term",
    async (locale) => {
      const { createTranslator } =
        await vi.importActual<typeof import("next-intl")>("next-intl");
      const messages = LOCALES[locale as keyof typeof LOCALES];
      const translate = createTranslator({
        locale,
        messages: messages as Record<string, unknown>,
        onError: (error) => {
          throw error;
        },
      }) as unknown as (key: string, values: Record<string, unknown>) => string;
      for (const namespace of NAMESPACES) {
        for (const [key, text] of leaves(messages[namespace], namespace)) {
          expect(text, key).not.toMatch(/\bOGS\b/);
          if (!text.includes("{careTerm}")) continue;
          const values: Record<string, unknown> = { careTerm: "GTS" };
          for (const [, name] of text.matchAll(/\{(\w+)/g)) {
            if (name && name !== "careTerm") values[name] = 1;
          }
          const formatted = translate(key, values);
          expect(formatted, key).toContain("GTS");
          expect(formatted, key).not.toContain("{careTerm}");
        }
      }
      for (const key of germanKeys) {
        const text = leaves(messages, "").find(([k]) => k === key)?.[1];
        expect(text, `${locale} ${key}`).toContain("{careTerm}");
      }
    },
  );
});
