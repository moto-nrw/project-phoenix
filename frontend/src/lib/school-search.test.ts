import { describe, expect, it, vi } from "vitest";

const { mockBffJson } = vi.hoisted(() => ({ mockBffJson: vi.fn() }));
vi.mock("~/lib/bff-fetch", () => ({ bffJson: mockBffJson }));

import {
  answerQuestionLabel,
  looksLikePhoneNumber,
  searchSchool,
} from "./school-search";

describe("looksLikePhoneNumber", () => {
  it("reads numbers the way the backend does", () => {
    for (const text of [
      "0170 1234567",
      "+49 170 1234567",
      "(0170) 123-45 67",
      "4567",
    ]) {
      expect(looksLikePhoneNumber(text)).toBe(true);
    }
    for (const text of [
      "Müller",
      "3a",
      "123",
      "0170 Müller",
      "mia@example.org",
    ]) {
      expect(looksLikePhoneNumber(text)).toBe(false);
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
