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

    // #2519: an unknown school is an error the page shows, not "no schema".
    it("rethrows a 404", async () => {
      const error = new OperatorApiError("not found", 404);
      mockOperatorFetch.mockRejectedValue(error);

      await expect(fetchOperatorSettingsSchema(SCHOOL_ID)).rejects.toBe(error);
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
    it("resolves on success", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await expect(
        setOperatorSettingValue(
          SCHOOL_ID,
          "operations.session_end_time",
          "18:30",
        ),
      ).resolves.toBeUndefined();
      expect(mockOperatorFetch).toHaveBeenCalledWith(
        `/api/operator/provisioning/schools/${SCHOOL_ID}/settings/values/operations.session_end_time`,
        { method: "PUT", body: { value: "18:30" } },
      );
    });

    // The field shows the error on the shared path (#2519); the client no
    // longer turns the backend sentence into its own text.
    it.each([
      ["validation", new OperatorApiError("below minimum 5", 400)],
      ["not found", new OperatorApiError("not found", 404)],
      ["conflict", new OperatorApiError("presence mode conflict", 409)],
      ["server", new OperatorApiError("boom", 500)],
      ["network", new Error("fetch failed")],
    ])("rethrows the %s error unchanged", async (_, error) => {
      mockOperatorFetch.mockRejectedValue(error);

      await expect(
        setOperatorSettingValue(SCHOOL_ID, "foo.bar", "x"),
      ).rejects.toBe(error);
    });
  });

  describe("resetOperatorSettingValue", () => {
    it("resolves on success", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await expect(
        resetOperatorSettingValue(SCHOOL_ID, "foo.bar"),
      ).resolves.toBeUndefined();
      expect(mockOperatorFetch).toHaveBeenCalledWith(
        `/api/operator/provisioning/schools/${SCHOOL_ID}/settings/values/foo.bar`,
        { method: "DELETE" },
      );
    });

    it.each([
      ["server", new OperatorApiError("boom", 500)],
      ["network", new Error("network down")],
    ])("rethrows the %s error unchanged", async (_, error) => {
      mockOperatorFetch.mockRejectedValue(error);

      await expect(
        resetOperatorSettingValue(SCHOOL_ID, "foo.bar"),
      ).rejects.toBe(error);
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

    // A failed reveal used to look like an empty value; the field now shows
    // it (#2519).
    it("rethrows a failed request", async () => {
      const error = new OperatorApiError("not found", 404);
      mockOperatorFetch.mockRejectedValue(error);

      await expect(
        revealOperatorSettingValue(SCHOOL_ID, "foo.bar"),
      ).rejects.toBe(error);
    });
  });
});
