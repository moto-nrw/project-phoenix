import { describe, it, expect, vi } from "vitest";
import {
  DEFAULT_PRIVACY_CONSENT,
  fetchPrivacyConsent,
  shouldCreatePrivacyConsent,
  updatePrivacyConsent,
} from "./student-privacy-helpers";

describe("student-privacy-helpers", () => {
  describe("DEFAULT_PRIVACY_CONSENT", () => {
    it("should have correct default values", () => {
      expect(DEFAULT_PRIVACY_CONSENT).toEqual({
        privacy_consent_accepted: false,
        data_retention_days: 30,
      });
    });
  });

  describe("fetchPrivacyConsent", () => {
    it("should fetch and return wrapped response", async () => {
      const apiGet = vi.fn().mockResolvedValue({
        data: {
          accepted: true,
          data_retention_days: 15,
        },
      });

      const result = await fetchPrivacyConsent("123", apiGet, "test-token");

      expect(apiGet).toHaveBeenCalledWith(
        "/api/students/123/privacy-consent",
        "test-token",
      );
      expect(result).toEqual({
        privacy_consent_accepted: true,
        data_retention_days: 15,
      });
    });

    it("should fetch and return unwrapped response", async () => {
      const apiGet = vi.fn().mockResolvedValue({
        accepted: false,
        data_retention_days: 30,
      });

      const result = await fetchPrivacyConsent("456", apiGet, "test-token");

      expect(result).toEqual({
        privacy_consent_accepted: false,
        data_retention_days: 30,
      });
    });

    it("should return defaults on 404 error", async () => {
      const apiGet = vi.fn().mockRejectedValue(new Error("Not found (404)"));

      const result = await fetchPrivacyConsent("789", apiGet, "test-token");

      expect(result).toEqual(DEFAULT_PRIVACY_CONSENT);
    });

    it("should return defaults on 403 error", async () => {
      const apiGet = vi.fn().mockRejectedValue(new Error("Forbidden (403)"));

      const result = await fetchPrivacyConsent("101", apiGet, "test-token");

      expect(result).toEqual(DEFAULT_PRIVACY_CONSENT);
    });

    it("should re-throw other errors", async () => {
      const apiGet = vi
        .fn()
        .mockRejectedValue(new Error("Internal Server Error (500)"));

      await expect(
        fetchPrivacyConsent("202", apiGet, "test-token"),
      ).rejects.toThrow("Internal Server Error (500)");
    });

    it("should re-throw non-Error throws", async () => {
      const apiGet = vi.fn().mockRejectedValue("string error");

      await expect(
        fetchPrivacyConsent("303", apiGet, "test-token"),
      ).rejects.toBe("string error");
    });

    it("should return defaults for null response", async () => {
      const apiGet = vi.fn().mockResolvedValue(null);

      const result = await fetchPrivacyConsent("404", apiGet, "test-token");

      expect(result).toEqual(DEFAULT_PRIVACY_CONSENT);
    });

    it("should return defaults for invalid response structure", async () => {
      const apiGet = vi.fn().mockResolvedValue({ invalid: "structure" });

      const result = await fetchPrivacyConsent("505", apiGet, "test-token");

      expect(result).toEqual(DEFAULT_PRIVACY_CONSENT);
    });

    it("should return defaults for wrapped response with invalid data", async () => {
      const apiGet = vi.fn().mockResolvedValue({
        data: { invalid: "fields" },
      });

      const result = await fetchPrivacyConsent("606", apiGet, "test-token");

      expect(result).toEqual(DEFAULT_PRIVACY_CONSENT);
    });

    it("should return defaults for wrapped response with non-boolean accepted", async () => {
      const apiGet = vi.fn().mockResolvedValue({
        data: {
          accepted: "true",
          data_retention_days: 15,
        },
      });

      const result = await fetchPrivacyConsent("707", apiGet, "test-token");

      expect(result).toEqual(DEFAULT_PRIVACY_CONSENT);
    });

    it("should return defaults for wrapped response with non-number retention days", async () => {
      const apiGet = vi.fn().mockResolvedValue({
        data: {
          accepted: true,
          data_retention_days: "15",
        },
      });

      const result = await fetchPrivacyConsent("808", apiGet, "test-token");

      expect(result).toEqual(DEFAULT_PRIVACY_CONSENT);
    });
  });

  describe("shouldCreatePrivacyConsent", () => {
    it("should return true when accepted is true", () => {
      expect(shouldCreatePrivacyConsent(true, 30)).toBe(true);
    });

    it("should return true when retention days is not 30", () => {
      expect(shouldCreatePrivacyConsent(false, 15)).toBe(true);
      expect(shouldCreatePrivacyConsent(false, 60)).toBe(true);
      expect(shouldCreatePrivacyConsent(undefined, 10)).toBe(true);
    });

    it("should return false when accepted is false and retention is 30", () => {
      expect(shouldCreatePrivacyConsent(false, 30)).toBe(false);
    });

    it("should return false when both are undefined", () => {
      expect(shouldCreatePrivacyConsent(undefined, undefined)).toBe(false);
    });

    it("should return false when accepted is undefined and retention is 30", () => {
      expect(shouldCreatePrivacyConsent(undefined, 30)).toBe(false);
    });

    it("should return false when retention is null", () => {
      expect(shouldCreatePrivacyConsent(false, null as unknown as number)).toBe(
        false,
      );
    });
  });

  describe("updatePrivacyConsent", () => {
    it("should call apiPut with correct payload for all parameters", async () => {
      const apiPut = vi.fn().mockResolvedValue({});

      await updatePrivacyConsent("123", apiPut, "test-token", true, 15);

      expect(apiPut).toHaveBeenCalledWith(
        "/api/students/123/privacy-consent",
        "test-token",
        {
          policy_version: "1.0",
          accepted: true,
          data_retention_days: 15,
        },
      );
    });

    it("should use default false for accepted when undefined", async () => {
      const apiPut = vi.fn().mockResolvedValue({});

      await updatePrivacyConsent("456", apiPut, "test-token", undefined, 20);

      expect(apiPut).toHaveBeenCalledWith(
        "/api/students/456/privacy-consent",
        "test-token",
        {
          policy_version: "1.0",
          accepted: false,
          data_retention_days: 20,
        },
      );
    });

    it("should use default 30 for retention days when undefined", async () => {
      const apiPut = vi.fn().mockResolvedValue({});

      await updatePrivacyConsent("789", apiPut, "test-token", true, undefined);

      expect(apiPut).toHaveBeenCalledWith(
        "/api/students/789/privacy-consent",
        "test-token",
        {
          policy_version: "1.0",
          accepted: true,
          data_retention_days: 30,
        },
      );
    });

    it("should handle number studentId", async () => {
      const apiPut = vi.fn().mockResolvedValue({});

      await updatePrivacyConsent(999, apiPut, "test-token", false, 30);

      expect(apiPut).toHaveBeenCalledWith(
        "/api/students/999/privacy-consent",
        "test-token",
        {
          policy_version: "1.0",
          accepted: false,
          data_retention_days: 30,
        },
      );
    });

    it("should handle custom operation name parameter", async () => {
      const apiPut = vi.fn().mockResolvedValue({});

      await updatePrivacyConsent(
        "111",
        apiPut,
        "test-token",
        true,
        30,
        "CustomOperation",
      );

      expect(apiPut).toHaveBeenCalled();
    });
  });
});
