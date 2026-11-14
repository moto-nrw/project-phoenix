import { describe, expect, it, vi } from "vitest";

const { mockBffJson } = vi.hoisted(() => ({ mockBffJson: vi.fn() }));
vi.mock("~/lib/bff-fetch", () => ({ bffJson: mockBffJson }));

import {
  answerQuestionLabel,
  looksLikePhoneNumber,
  phoneSearchKey,
  searchSchool,
} from "./school-search";

// The same cases as the backend's PhoneSearchKey tests
// (schoolsearchview_test.go): both sides must read a number alike.
const PHONE_KEYS: ReadonlyArray<readonly [string, string]> = [
  ["+49 170 1234567", "1701234567"],
  ["+491701234567", "1701234567"],
  ["0049 170 1234567", "1701234567"],
  ["0049-170-1234567", "1701234567"],
  ["+49 (0)170 1234567", "1701234567"],
  ["0170 1234567", "1701234567"],
  ["0170/1234567", "1701234567"],
  ["0170-123 45 67", "1701234567"],
  ["(0170) 123.45.67", "1701234567"],
  ["  0221 98765  ", "22198765"],
  ["+43 1 2345678", "4312345678"],
  ["0043 1 2345678", "4312345678"],
  ["4567", "4567"],
  ["0170\u00a01234567", "1701234567"],
  ["0170\u202f123\u00a045\u00a067", "1701234567"],
  ["0170\t1234567", "1701234567"],
  ["0170\u20131234567", "1701234567"],
  ["0170\u20111234567", "1701234567"],
  ["0170\u22121234567", "1701234567"],
  ["\u00a0+49\u00a0170\u00a01234567", "1701234567"],
  ["+43 (0)1 2345678", "4312345678"],
  ["0043 (0) 1 2345678", "4312345678"],
  ["+43 (\u00a00)1 2345678", "4312345678"],
  ["+43 (\u20020\t)1 2345678", "4312345678"],
  // A 49 typed without + or 00 stays (Emden's 04921 without its 0); the
  // backend adds the reading as a country code as a second key.
  ["49 221 5550001", "492215550001"],
  ["4930 1234", "49301234"],
  ["04921 1234567", "49211234567"],
  ["4921 1234567", "49211234567"],
  ["(+49) 170 1234567", "1701234567"],
  ["+49 1234", "1234"],
];

const NOT_PHONE_NUMBERS = [
  "Müller",
  "3a",
  "123",
  "0170 Müller",
  "mia@example.org",
  "+49",
  "12-3",
  "0170_1234567",
  "0170·1234567",
  "0170",
  "+49 12",
  "0049 123",
  "(0)123",
  "+49 (0)12",
];

describe("phoneSearchKey", () => {
  it("reads every way of writing a number like the backend", () => {
    for (const [text, key] of PHONE_KEYS) {
      expect(phoneSearchKey(text), JSON.stringify(text)).toBe(key);
    }
    for (const text of NOT_PHONE_NUMBERS) {
      expect(phoneSearchKey(text), text).toBe("");
    }
  });
});

describe("looksLikePhoneNumber", () => {
  it("reads numbers the way the backend does", () => {
    for (const [text] of PHONE_KEYS) {
      expect(looksLikePhoneNumber(text), JSON.stringify(text)).toBe(true);
    }
    for (const text of NOT_PHONE_NUMBERS) {
      expect(looksLikePhoneNumber(text), text).toBe(false);
    }
  });
});

describe("searchSchool", () => {
  it("posts the query in the body, never in the address", async () => {
    mockBffJson.mockResolvedValueOnce({});
    const controller = new AbortController();
    await searchSchool("0170 1234567", controller.signal);
    expect(mockBffJson).toHaveBeenCalledWith("/api/search", {
      method: "POST",
      body: { query: "0170 1234567" },
      signal: controller.signal,
    });
  });
});

describe("answerQuestionLabel", () => {
  it("drops the question's own end mark before its answer", () => {
    expect(answerQuestionLabel("Mein Kind darf schwimmen.")).toBe(
      "Mein Kind darf schwimmen",
    );
    expect(answerQuestionLabel("Seepferdchen vorhanden? ")).toBe(
      "Seepferdchen vorhanden",
    );
    expect(answerQuestionLabel("Wunsch-AG")).toBe("Wunsch-AG");
  });
});
