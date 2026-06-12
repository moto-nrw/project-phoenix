import { describe, it, expect } from "vitest";
import {
  localeFromAcceptLanguage,
  mergeMessages,
  resolveLocale,
} from "./locale-resolution";

describe("mergeMessages", () => {
  it("recurses into nested objects so a missing key inherits the fallback", () => {
    const base = { a: { x: "de-x", y: "de-y" }, b: "de-b" };
    const override = { a: { x: "en-x" } };
    expect(mergeMessages(base, override)).toEqual({
      a: { x: "en-x", y: "de-y" }, // y inherited from base
      b: "de-b",
    });
  });

  it("replaces primitives and arrays outright (no array merge)", () => {
    const base = { list: ["de1", "de2"], n: 1 };
    const override = { list: ["en1"], n: 2 };
    expect(mergeMessages(base, override)).toEqual({ list: ["en1"], n: 2 });
  });

  it("does not mutate the base object", () => {
    const base = { a: { x: "de" } };
    mergeMessages(base, { a: { x: "en" } });
    expect(base).toEqual({ a: { x: "de" } });
  });
});

describe("localeFromAcceptLanguage", () => {
  it("returns the default for a null or empty header", () => {
    expect(localeFromAcceptLanguage(null)).toBe("de");
    expect(localeFromAcceptLanguage("")).toBe("de");
  });

  it("picks the first supported non-default candidate", () => {
    expect(localeFromAcceptLanguage("en-US,en;q=0.9")).toBe("en");
    expect(localeFromAcceptLanguage("ru,en;q=0.8")).toBe("ru");
  });

  it("respects q-values before header order", () => {
    expect(localeFromAcceptLanguage("en;q=0,de;q=1")).toBe("de");
    expect(localeFromAcceptLanguage("de;q=0.7,en;q=0.8")).toBe("en");
    expect(localeFromAcceptLanguage("en;q=0,ru;q=0.5")).toBe("ru");
  });

  it("keeps header order as the q-value tie-breaker", () => {
    expect(localeFromAcceptLanguage("ru;q=0.8,en;q=0.8")).toBe("ru");
    expect(localeFromAcceptLanguage("en;q=0.8,ru;q=0.8")).toBe("en");
  });

  it("ignores malformed q-values", () => {
    expect(localeFromAcceptLanguage("en;q=2,ru;q=0.5")).toBe("ru");
    expect(localeFromAcceptLanguage("en;q=0.8junk,ru;q=0.5")).toBe("ru");
  });

  it("skips unsupported leading candidates", () => {
    // fr is unsupported → normalizes to default but is not a `de` tag, so the
    // loop continues to en.
    expect(localeFromAcceptLanguage("fr-FR,en;q=0.7")).toBe("en");
  });

  it("honours an explicit German preference instead of falling through", () => {
    expect(localeFromAcceptLanguage("de-DE,en;q=0.9")).toBe("de");
    expect(localeFromAcceptLanguage("de")).toBe("de");
  });

  it("returns the default when no candidate is supported", () => {
    expect(localeFromAcceptLanguage("fr-FR,es;q=0.8")).toBe("de");
  });
});

describe("resolveLocale", () => {
  it("forces the default on a non-localized surface, ignoring cookie + header", () => {
    expect(
      resolveLocale({
        localized: false,
        cookieValue: "en",
        acceptLanguage: "ru",
      }),
    ).toBe("de");
  });

  it("prefers the cookie on a localized surface", () => {
    expect(
      resolveLocale({
        localized: true,
        cookieValue: "ru",
        acceptLanguage: "en",
      }),
    ).toBe("ru");
  });

  it("normalizes a region-tagged cookie value", () => {
    expect(
      resolveLocale({
        localized: true,
        cookieValue: "en-US",
        acceptLanguage: null,
      }),
    ).toBe("en");
  });

  it("falls back to Accept-Language when no cookie is set", () => {
    expect(
      resolveLocale({
        localized: true,
        cookieValue: undefined,
        acceptLanguage: "sq,en;q=0.5",
      }),
    ).toBe("sq");
  });
});
