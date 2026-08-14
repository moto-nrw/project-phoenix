import { describe, expect, it } from "vitest";

import {
  encodeMultiValueParam,
  normalizeMultiValues,
  parseMultiValueParam,
} from "./multi-value-param";

describe("parseMultiValueParam", () => {
  it("splits a plain comma list", () => {
    expect(parseMultiValueParam("3a,4b")).toEqual(["3a", "4b"]);
  });

  it("trims, drops blanks and collapses duplicates", () => {
    expect(parseMultiValueParam(" 3a , ,3a, 4b ")).toEqual(["3a", "4b"]);
  });

  it("treats an absent parameter as no selection", () => {
    expect(parseMultiValueParam(null)).toEqual([]);
    expect(parseMultiValueParam("")).toEqual([]);
  });

  // A class may be called "A,B" — the separator then belongs to the value and
  // must not split it (#2218 review).
  it("keeps an escaped comma inside its value", () => {
    expect(parseMultiValueParam("A\\,B")).toEqual(["A,B"]);
    expect(parseMultiValueParam("A\\,B,3a")).toEqual(["A,B", "3a"]);
  });

  it("unescapes a literal backslash", () => {
    expect(parseMultiValueParam("A\\\\B")).toEqual(["A\\B"]);
    expect(parseMultiValueParam("A\\\\,3a")).toEqual(["A\\", "3a"]);
  });
});

describe("encodeMultiValueParam", () => {
  it("leaves ordinary values untouched, so old links keep working", () => {
    expect(encodeMultiValueParam(["3a", "4b"])).toBe("3a,4b");
  });

  it("renders an empty selection as an empty parameter", () => {
    expect(encodeMultiValueParam([])).toBe("");
  });

  it("escapes the separator and the escape character", () => {
    expect(encodeMultiValueParam(["A,B"])).toBe("A\\,B");
    expect(encodeMultiValueParam(["A\\B"])).toBe("A\\\\B");
  });

  // The round trip is what the URL, the stored filters, the SWR cache key and
  // the export request all depend on.
  it("round-trips every selection", () => {
    for (const selection of [
      ["3a", "4b"],
      ["A,B"],
      ["A,B", "A", "B"],
      ["A\\,B"],
      ["Klasse 3a", "Bienen"],
    ]) {
      expect(parseMultiValueParam(encodeMultiValueParam(selection))).toEqual(
        selection,
      );
    }
  });

  // Without escaping these two selections encode identically, which is both a
  // wrong filter and a cache key collision.
  it("keeps one comma-carrying class apart from two classes", () => {
    expect(encodeMultiValueParam(["A,B"])).not.toBe(
      encodeMultiValueParam(["A", "B"]),
    );
  });
});

describe("normalizeMultiValues", () => {
  it("trims, drops blanks and collapses duplicates without splitting", () => {
    expect(normalizeMultiValues([" A,B ", "A,B", "", "3a"])).toEqual([
      "A,B",
      "3a",
    ]);
  });
});
