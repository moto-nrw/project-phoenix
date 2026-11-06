import { describe, expect, it } from "vitest";

import { foldForSearch } from "./search-fold";

describe("foldForSearch", () => {
  it.each([
    ["Gómez", "gomez"],
    ["Özdemir", "ozdemir"],
    ["Łukasz Ćirić", "lukasz ciric"],
    ["ŁUKASZ", "lukasz"],
    ["Søren Ø", "soren o"],
    ["Đoković Đ", "dokovic d"],
    ["Straße", "strasse"],
    ["Iİı", "iii"],
    ["Zoë", "zoe"],
    ["Müller", "muller"],
    ["Raum 2b", "raum 2b"],
    ["Cœur Æsir ǽ", "coeur aesir ae"],
  ])("folds %s to %s", (input, expected) => {
    expect(foldForSearch(input)).toBe(expected);
  });

  it("finds an accented name for a query typed without accents", () => {
    expect(foldForSearch("Zoë Özdemir").includes(foldForSearch("ozdem"))).toBe(
      true,
    );
    expect(foldForSearch("Gómez").includes(foldForSearch("GOMEZ"))).toBe(true);
  });

  it("does not turn umlauts into two letters", () => {
    expect(foldForSearch("Müller").includes(foldForSearch("mueller"))).toBe(
      false,
    );
  });

  it("treats a missing value as empty", () => {
    expect(foldForSearch(null)).toBe("");
    expect(foldForSearch(undefined)).toBe("");
  });
});
