import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockApiGet, mockApiPut } = vi.hoisted(() => ({
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPut: mockApiPut,
}));

import { ApiResponseError } from "~/lib/api-response-error.server";
import {
  readConsent,
  readConsentAfterWrite,
  sentConsentPair,
  writeConsent,
} from "./student-privacy-consent.server";

beforeEach(() => {
  mockApiGet.mockReset();
  mockApiPut.mockReset();
});

describe("sentConsentPair", () => {
  it("is null when the request carries neither field", () => {
    expect(sentConsentPair({})).toBeNull();
  });

  it("returns the pair when both fields travel", () => {
    expect(
      sentConsentPair({
        privacy_consent_accepted: true,
        data_retention_days: 14,
      }),
    ).toEqual({ privacy_consent_accepted: true, data_retention_days: 14 });
  });

  it("refuses a lone field with 400, since the write would reset the other", () => {
    for (const body of [
      { privacy_consent_accepted: true },
      { data_retention_days: 14 },
    ]) {
      expect(() => sentConsentPair(body)).toThrow(
        expect.objectContaining({ status: 400 }),
      );
    }
  });
});

describe("writeConsent", () => {
  it("upserts the pair with the policy version", async () => {
    mockApiPut.mockResolvedValue({});

    await writeConsent("7", "token", {
      privacy_consent_accepted: false,
      data_retention_days: 21,
    });

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/7/privacy-consent",
      "token",
      { policy_version: "1.0", accepted: false, data_retention_days: 21 },
    );
  });
});

describe("readConsent", () => {
  it("reads the stored consent or the school's defaults", async () => {
    mockApiGet.mockResolvedValue({
      data: { accepted: false, data_retention_days: 12 },
    });

    await expect(readConsent("7", "token")).resolves.toEqual({
      privacy_consent_accepted: false,
      data_retention_days: 12,
    });
  });

  it("is null when the caller may not read the consent", async () => {
    mockApiGet.mockRejectedValue(new ApiResponseError(403, "{}"));

    await expect(readConsent("7", "token")).resolves.toBeNull();
  });

  it("propagates every other failure", async () => {
    mockApiGet.mockRejectedValue(new ApiResponseError(500, "{}"));

    await expect(readConsent("7", "token")).rejects.toMatchObject({
      status: 500,
    });
  });
});

describe("readConsentAfterWrite", () => {
  it("falls back to the pair just written when the read-back fails", async () => {
    mockApiGet.mockRejectedValue(new ApiResponseError(500, "{}"));
    const written = { privacy_consent_accepted: true, data_retention_days: 9 };

    await expect(readConsentAfterWrite("7", "token", written)).resolves.toBe(
      written,
    );
    await expect(readConsentAfterWrite("7", "token", null)).resolves.toBeNull();
  });
});
