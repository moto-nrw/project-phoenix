import { describe, it, expect, beforeEach } from "vitest";
import {
  DEFAULT_LOCALE,
  LOCALE_COOKIE_NAME,
  normalizeLocale,
  SUPPORTED_LOCALES,
  writeLocaleCookie,
} from "./locales";

describe("DEFAULT_LOCALE", () => {
  it("is the locale flagged as fallback in the shared source of truth", () => {
    // de is marked `fallback: true` in backend/localization/locales.json,
    // mirrored into locales.generated.ts.
    expect(DEFAULT_LOCALE).toBe("de");
    expect(SUPPORTED_LOCALES.some((l) => l.code === DEFAULT_LOCALE)).toBe(true);
  });
});

describe("normalizeLocale", () => {
  it("passes a registered locale through unchanged", () => {
    expect(normalizeLocale("en")).toBe("en");
    expect(normalizeLocale("ru")).toBe("ru");
    expect(normalizeLocale("sq")).toBe("sq");
  });

  it("lowercases", () => {
    expect(normalizeLocale("EN")).toBe("en");
  });

  it("strips a region subtag (hyphen or underscore)", () => {
    expect(normalizeLocale("en-US")).toBe("en");
    expect(normalizeLocale("en_GB")).toBe("en");
  });

  it("trims surrounding whitespace", () => {
    expect(normalizeLocale("  en  ")).toBe("en");
  });

  it("coerces an unknown locale to the default", () => {
    expect(normalizeLocale("xx")).toBe(DEFAULT_LOCALE);
    expect(normalizeLocale("fr-FR")).toBe(DEFAULT_LOCALE);
  });

  it("coerces empty / nullish input to the default", () => {
    expect(normalizeLocale("")).toBe(DEFAULT_LOCALE);
    expect(normalizeLocale("   ")).toBe(DEFAULT_LOCALE);
    expect(normalizeLocale(null)).toBe(DEFAULT_LOCALE);
    expect(normalizeLocale(undefined)).toBe(DEFAULT_LOCALE);
  });
});

describe("writeLocaleCookie", () => {
  beforeEach(() => {
    // Clear any cookie left by a previous test.
    document.cookie = `${LOCALE_COOKIE_NAME}=; path=/; max-age=0`;
  });

  it("writes the locale to the parent-locale cookie", () => {
    writeLocaleCookie("en");
    expect(document.cookie).toContain(`${LOCALE_COOKIE_NAME}=en`);
  });

  it("url-encodes the value", () => {
    // sq is plain ASCII; assert encoding is applied without corrupting it.
    writeLocaleCookie("sq");
    expect(document.cookie).toContain(`${LOCALE_COOKIE_NAME}=sq`);
  });
});
