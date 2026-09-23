import { describe, expect, it } from "vitest";

import {
  CHILD_QUOTA_REACHED_CODE,
  childQuotaMessage,
} from "./child-quota-error";

function codedError(code: string, details?: unknown) {
  return Object.assign(new Error("child quota reached"), { code, details });
}

describe("childQuotaMessage", () => {
  it("names the Kinderkontingent with the booked and occupied children", () => {
    const error = codedError(CHILD_QUOTA_REACHED_CODE, {
      booked_places: 50,
      occupied_places: 50,
      requested_places: 1,
    });

    expect(childQuotaMessage(error)).toBe(
      "Das Kinderkontingent Ihrer Schule ist voll. Die Kontingentzahl beträgt 50 von 50 Kindern. Für weitere Kinder melden Sie sich bitte beim moto-Team.",
    );
  });

  it("reads the code from the raw response body the CRUD fetch keeps", () => {
    const error = Object.assign(new Error("HTTP 409"), {
      status: 409,
      body: JSON.stringify({
        error: "child quota reached",
        code: CHILD_QUOTA_REACHED_CODE,
        details: { booked_places: 100, occupied_places: 102 },
      }),
    });

    expect(childQuotaMessage(error)).toContain(
      "Die Kontingentzahl beträgt 102 von 100 Kindern.",
    );
  });

  it("stays readable without numbers", () => {
    expect(childQuotaMessage(codedError(CHILD_QUOTA_REACHED_CODE))).toBe(
      "Das Kinderkontingent Ihrer Schule ist voll. Für weitere Kinder melden Sie sich bitte beim moto-Team.",
    );
  });

  it("ignores every other error", () => {
    expect(childQuotaMessage(codedError("students.other"))).toBeNull();
    expect(childQuotaMessage(new Error("child quota reached"))).toBeNull();
    expect(
      childQuotaMessage(Object.assign(new Error("x"), { body: "not json" })),
    ).toBeNull();
    expect(childQuotaMessage(undefined)).toBeNull();
  });
});
