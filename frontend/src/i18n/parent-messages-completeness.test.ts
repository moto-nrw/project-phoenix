import { describe, expect, it } from "vitest";
import {
  ERROR_CODES,
  ERROR_CODE_CLASSES,
  type ErrorClass,
} from "~/lib/error-codes.generated";
import de from "./messages/de.json";
import en from "./messages/en.json";
import pl from "./messages/pl.json";
import ru from "./messages/ru.json";
import sq from "./messages/sq.json";
import tr from "./messages/tr.json";
import uk from "./messages/uk.json";
import { PARENT_PORTAL_ERROR_CODES } from "~/test/parent-error-codes";

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

// en, ru and sq are complete as well since #2518: every error object phrase
// and local error text parents can read is guarded, not only the catalog.
// The exceptions are words that are spelled the same in that language, named
// by key so a new German leftover cannot hide behind them.
const sameWordByLocale: Record<"en" | "ru" | "sq", ReadonlySet<string>> = {
  en: new Set([
    "parentNav.feedback",
    "parentFeedback.title",
    "parentDeclaration.confirmVersion",
    "enrollmentForm.actions.details",
    "enrollmentStatus.nameLabel",
    "enrollmentStatus.changeRequestAuthor.system",
    "parentDashboard.newsPollChildAnswer",
    // April, August, September, November
    "enrollmentForm.months.3",
    "enrollmentForm.months.7",
    "enrollmentForm.months.8",
    "enrollmentForm.months.10",
  ]),
  ru: new Set(["parentDashboard.newsPollChildAnswer"]),
  sq: new Set([
    "parentDashboard.newsPollChildAnswer",
    "parentChildDetail.guardians.relationships.contact",
    "enrollmentForm.phoneTypes.home",
  ]),
};

describe("complete catalogs for en, ru and sq", () => {
  it.each([
    ["en", en],
    ["ru", ru],
    ["sq", sq],
  ] as const)("has no German copy left in %s", (locale, catalog) => {
    const german = flatten(de as unknown as MessageTree);
    const translated = flatten(catalog as unknown as MessageTree);
    const untranslated = Object.entries(translated).filter(
      ([key, value]) =>
        value === german[key] &&
        !invariantValues.has(value) &&
        !fullCatalogInvariants.has(value) &&
        !sameWordByLocale[locale].has(key),
    );

    expect(untranslated).toEqual([]);
  });
});

// Error texts the parents portal can show (#2518). A code with its own German
// sentence names a specific next step; a locale that only has the class text
// for it would hide that step from families reading another language.
const errorLocales = [
  ["en", en],
  ["ru", ru],
  ["sq", sq],
  ["pl", pl],
  ["tr", tr],
  ["uk", uk],
] as const;

interface ErrorCatalogTree {
  classes: Record<ErrorClass, string>;
  actions: Record<string, string>;
  loginNotice: string;
  codes: Record<string, Record<string, string>>;
}

function codeText(catalog: ErrorCatalogTree, code: string): string | undefined {
  const [area, name] = code.split(".") as [string, string];
  return catalog.codes[area]?.[name];
}

describe("parent portal error texts", () => {
  const german = de.errorCatalog as ErrorCatalogTree;

  it.each(errorLocales)(
    "translates the class texts, actions and login notice in %s",
    (_locale, messages) => {
      const catalog = messages.errorCatalog as ErrorCatalogTree;
      const untranslated = [
        ...Object.entries(catalog.classes).map(([key, value]) => [
          `classes.${key}`,
          value === german.classes[key as ErrorClass],
        ]),
        ...Object.entries(catalog.actions).map(([key, value]) => [
          `actions.${key}`,
          value === german.actions[key],
        ]),
        ["loginNotice", catalog.loginNotice === german.loginNotice],
      ]
        .filter(([, same]) => same)
        .map(([key]) => key);

      expect(untranslated).toEqual([]);
    },
  );

  it.each(errorLocales)(
    "gives every parent code with its own German text its own text in %s",
    (_locale, messages) => {
      const catalog = messages.errorCatalog as ErrorCatalogTree;
      const missing = PARENT_PORTAL_ERROR_CODES.filter((code) => {
        const errorClass = ERROR_CODE_CLASSES[code];
        const germanText = codeText(german, code);
        if (germanText === german.classes[errorClass]) return false;
        const text = codeText(catalog, code);
        // Any class text counts as missing: a code moved to another class
        // can keep the old class sentence and read as the wrong next step.
        return (
          !text ||
          Object.values(catalog.classes).includes(text) ||
          text === germanText
        );
      });

      expect(missing).toEqual([]);
    },
  );

  it("lists only registered codes", () => {
    const registered: ReadonlySet<string> = new Set(ERROR_CODES);
    expect(
      PARENT_PORTAL_ERROR_CODES.filter((code) => !registered.has(code)),
    ).toEqual([]);
  });
});
