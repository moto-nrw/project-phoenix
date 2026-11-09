import { describe, expect, it } from "vitest";
import {
  type DocumentKindOverview,
  mergeSavedService,
} from "./traeger-document-kinds-api";

function overview(
  holiday: "confirmation" | "invoice",
  care: "invoice" | "fee_notice",
): DocumentKindOverview {
  return {
    services: [
      {
        service: "holiday",
        documentKind: holiday,
        choices: [],
        updatedAt: null,
      },
      { service: "care", documentKind: care, choices: [], updatedAt: null },
    ],
    legalRemedySet: true,
    feeNoticeTemplateVersion: 1,
  };
}

describe("mergeSavedService", () => {
  it("keeps a choice another card saved meanwhile (#4688)", () => {
    // The holiday card saved "invoice"; the answer to the monthly card
    // arrives later and still names the holiday choice of before.
    const shown = overview("invoice", "invoice");
    const lateAnswer = overview("confirmation", "fee_notice");
    expect(mergeSavedService(shown, lateAnswer, "care").services).toEqual(
      overview("invoice", "fee_notice").services,
    );
  });

  it("takes the answer as it is without a shown state", () => {
    const answer = overview("confirmation", "fee_notice");
    expect(mergeSavedService(null, answer, "care")).toBe(answer);
  });
});
