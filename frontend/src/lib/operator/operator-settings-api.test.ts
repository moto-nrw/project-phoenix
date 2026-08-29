import { describe, it, expect, vi, beforeEach } from "vitest";

// Hoist mocks so vi.mock() below can reference them
const { mockOperatorFetch } = vi.hoisted(() => ({
  mockOperatorFetch: vi.fn(),
}));

vi.mock("./api-helpers", () => ({
  operatorFetch: mockOperatorFetch,
  OperatorApiError: class OperatorApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.name = "OperatorApiError";
      this.status = status;
    }
  },
  isOperatorApiError: (e: unknown) =>
    e instanceof Error && e.name === "OperatorApiError",
}));

// Import after mocks are set up
import {
  fetchBookingAuthorityImpact,
  fetchOperatorSettingsSchema,
  setOperatorSettingValue,
  resetOperatorSettingValue,
  revealOperatorSettingValue,
} from "./operator-settings-api";
import { OperatorApiError } from "./api-helpers";

const SCHOOL_ID = "42";

describe("operator-settings-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("fetchBookingAuthorityImpact", () => {
    it("loads the school-scoped impact preview", async () => {
      const wire = {
        reference_date: "2026-08-25",
        blocking_children: [],
        planned_completions: [],
      };
      mockOperatorFetch.mockResolvedValue(wire);

      await expect(fetchBookingAuthorityImpact(SCHOOL_ID)).resolves.toEqual({
        referenceDate: "2026-08-25",
        blockingChildren: [],
        plannedCompletions: [],
      });
      expect(mockOperatorFetch).toHaveBeenCalledWith(
        `/api/operator/provisioning/schools/${SCHOOL_ID}/settings/booking-authority-impact`,
        { method: "GET" },
      );
    });
  });

  describe("fetchOperatorSettingsSchema", () => {
    it("calls the school-scoped schema endpoint and returns schema", async () => {
      const schema = {
        tabs: [{ key: "operations", label: "Ops", categories: [] }],
      };
      mockOperatorFetch.mockResolvedValue(schema);

      const result = await fetchOperatorSettingsSchema(SCHOOL_ID);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        `/api/operator/provisioning/schools/${SCHOOL_ID}/settings/schema`,
        { method: "GET" },
      );
      expect(result).toEqual(schema);
    });

    it("returns null on 404", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError("not found", 404),
      );

      const result = await fetchOperatorSettingsSchema(SCHOOL_ID);
      expect(result).toBeNull();
    });

    it("rethrows on non-404 error", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError("server error", 500),
      );

      await expect(fetchOperatorSettingsSchema(SCHOOL_ID)).rejects.toThrow(
        "server error",
      );
    });

    it("rethrows on generic Error", async () => {
      mockOperatorFetch.mockRejectedValue(new Error("network down"));

      await expect(fetchOperatorSettingsSchema(SCHOOL_ID)).rejects.toThrow(
        "network down",
      );
    });

    it("rethrows on non-Error rejection (e.g. string)", async () => {
      mockOperatorFetch.mockRejectedValue("raw-string-error");

      await expect(fetchOperatorSettingsSchema(SCHOOL_ID)).rejects.toBe(
        "raw-string-error",
      );
    });
  });

  describe("setOperatorSettingValue", () => {
    it("returns null on success", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      const err = await setOperatorSettingValue(
        SCHOOL_ID,
        "operations.session_end_time",
        "18:30",
      );

      expect(err).toBeNull();
      expect(mockOperatorFetch).toHaveBeenCalledWith(
        `/api/operator/provisioning/schools/${SCHOOL_ID}/settings/values/operations.session_end_time`,
        { method: "PUT", body: { value: "18:30" } },
      );
    });

    it("returns 'not found' message on 404", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError("not found", 404),
      );

      const err = await setOperatorSettingValue(SCHOOL_ID, "bad.key", "x");
      expect(err).toBe("Einstellung nicht gefunden.");
    });

    it("translates 'below minimum' validation error", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError(
          "invalid value for setting foo.bar: below minimum 5",
          400,
        ),
      );

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", 1);
      expect(err).toBe("Minimum: 5");
    });

    it("translates 'exceeds maximum' validation error", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError(
          "invalid value for setting foo.bar: exceeds maximum 60",
          400,
        ),
      );

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", 99);
      expect(err).toBe("Maximum: 60");
    });

    it("translates 'value is required' validation error", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError(
          "invalid value for setting foo.bar: value is required",
          400,
        ),
      );

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", "");
      expect(err).toBe("Dieser Wert ist erforderlich.");
    });

    it("translates 'expected a number' validation error", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError(
          "invalid value for setting foo.bar: expected a number",
          400,
        ),
      );

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", "abc");
      expect(err).toBe("Bitte eine Zahl eingeben.");
    });

    it("returns generic 'Ungültiger Wert' for unknown 400 reason", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError(
          "invalid value for setting foo.bar: something weird",
          400,
        ),
      );

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", "x");
      expect(err).toBe("Ungültiger Wert.");
    });

    it("returns generic 'Ungültiger Wert' for 400 with no colon separator", async () => {
      // Tests the translateValidationError fallback path where the backend
      // error string has no ": " separator and the whole message is used as
      // the reason.
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError("malformed-400-no-colon", 400),
      );

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", "x");
      expect(err).toBe("Ungültiger Wert.");
    });

    it("returns generic save error on 500", async () => {
      mockOperatorFetch.mockRejectedValue(new OperatorApiError("boom", 500));

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", "x");
      expect(err).toBe("Einstellung konnte nicht gespeichert werden.");
    });

    it("does not mislabel another setting's conflict as booking mode", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError("presence mode conflict", 409),
      );

      const err = await setOperatorSettingValue(
        SCHOOL_ID,
        "attendance.presence_mode",
        "binary",
      );
      expect(err).toBe("Einstellung konnte nicht gespeichert werden.");
    });

    it("returns network-error message for non-OperatorApiError", async () => {
      mockOperatorFetch.mockRejectedValue(new Error("fetch failed"));

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", "x");
      expect(err).toBe("Netzwerkfehler beim Speichern der Einstellung.");
    });

    it("returns network-error message for non-Error thrown value (e.g. string)", async () => {
      mockOperatorFetch.mockRejectedValue("raw-string-error");

      const err = await setOperatorSettingValue(SCHOOL_ID, "foo.bar", "x");
      expect(err).toBe("Netzwerkfehler beim Speichern der Einstellung.");
    });
  });

  describe("resetOperatorSettingValue", () => {
    it("returns null on success", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      const err = await resetOperatorSettingValue(SCHOOL_ID, "foo.bar");
      expect(err).toBeNull();
      expect(mockOperatorFetch).toHaveBeenCalledWith(
        `/api/operator/provisioning/schools/${SCHOOL_ID}/settings/values/foo.bar`,
        { method: "DELETE" },
      );
    });

    it("returns error message on failure", async () => {
      mockOperatorFetch.mockRejectedValue(new OperatorApiError("boom", 500));

      const err = await resetOperatorSettingValue(SCHOOL_ID, "foo.bar");
      expect(err).toBe("Einstellung konnte nicht zurückgesetzt werden.");
    });

    it("returns error message for non-OperatorApiError", async () => {
      mockOperatorFetch.mockRejectedValue(new Error("network down"));

      const err = await resetOperatorSettingValue(SCHOOL_ID, "foo.bar");
      expect(err).toBe("Einstellung konnte nicht zurückgesetzt werden.");
    });

    it("returns error message for non-Error thrown value", async () => {
      mockOperatorFetch.mockRejectedValue("raw-string-error");

      const err = await resetOperatorSettingValue(SCHOOL_ID, "foo.bar");
      expect(err).toBe("Einstellung konnte nicht zurückgesetzt werden.");
    });
  });

  describe("revealOperatorSettingValue", () => {
    it("returns string value on success", async () => {
      mockOperatorFetch.mockResolvedValue({ value: "1234" });

      const result = await revealOperatorSettingValue(
        SCHOOL_ID,
        "security.ogs_device_pin",
      );
      expect(result).toBe("1234");
      expect(mockOperatorFetch).toHaveBeenCalledWith(
        `/api/operator/provisioning/schools/${SCHOOL_ID}/settings/values/security.ogs_device_pin/reveal`,
        { method: "GET" },
      );
    });

    it("returns null when value is not a string", async () => {
      mockOperatorFetch.mockResolvedValue({ value: 1234 });

      const result = await revealOperatorSettingValue(SCHOOL_ID, "foo.bar");
      expect(result).toBeNull();
    });

    it("returns null when fetch throws", async () => {
      mockOperatorFetch.mockRejectedValue(
        new OperatorApiError("not found", 404),
      );

      const result = await revealOperatorSettingValue(SCHOOL_ID, "foo.bar");
      expect(result).toBeNull();
    });

    it("returns null when fetch throws non-Error value", async () => {
      mockOperatorFetch.mockRejectedValue("raw-string-error");

      const result = await revealOperatorSettingValue(SCHOOL_ID, "foo.bar");
      expect(result).toBeNull();
    });
  });
});
