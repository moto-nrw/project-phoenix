/**
 * Tests for group-transfer-api.ts
 *
 * Tests the groupTransferService functions:
 * - getStaffByRole
 * - transferGroup
 * - getActiveTransfersForGroup
 * - cancelTransferBySubstitutionId
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { MockInstance } from "vitest";

// Mock next-auth/react before importing the module
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(() =>
    Promise.resolve({
      user: { token: "test-token" },
    }),
  ),
}));

// Import after mocking
import { groupTransferService } from "./group-transfer-api";
import type { StaffWithRole, GroupTransfer } from "./group-transfer-api";

describe("groupTransferService", () => {
  let fetchMock: MockInstance<typeof fetch>;

  beforeEach(() => {
    vi.clearAllMocks();
    fetchMock = vi.spyOn(globalThis, "fetch");
  });

  afterEach(() => {
    fetchMock.mockRestore();
  });

  describe("getStaffByRole", () => {
    it("returns mapped staff list on success", async () => {
      const backendData = [
        {
          id: 1,
          person_id: 10,
          first_name: "Max",
          last_name: "Mustermann",
          full_name: "Max Mustermann",
          account_id: 100,
          email: "max@example.com",
        },
        {
          id: 2,
          person_id: 20,
          first_name: "Anna",
          last_name: "Schmidt",
          full_name: "Anna Schmidt",
          account_id: 200,
          email: "anna@example.com",
        },
      ];

      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: backendData }),
      } as Response);

      const result = await groupTransferService.getStaffByRole("teacher");

      expect(result).toHaveLength(2);
      expect(result[0]).toEqual<StaffWithRole>({
        id: "1",
        personId: "10",
        firstName: "Max",
        lastName: "Mustermann",
        fullName: "Max Mustermann",
        accountId: "100",
        email: "max@example.com",
      });
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/staff/by-role?role=teacher",
        expect.objectContaining({
          credentials: "include",
        }),
      );
    });

    it("returns empty array when data is null", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: null }),
      } as Response);

      const result = await groupTransferService.getStaffByRole("staff");

      expect(result).toEqual([]);
    });

    it("throws error on fetch failure", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 500,
      } as Response);

      await expect(
        groupTransferService.getStaffByRole("teacher"),
      ).rejects.toThrow("Laden der Betreuer fehlgeschlagen");
    });
  });

  describe("transferGroup", () => {
    it("completes successfully", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true }),
      } as Response);

      await expect(
        groupTransferService.transferGroup("123", "456"),
      ).resolves.toBeUndefined();

      expect(fetchMock).toHaveBeenCalledWith(
        "/api/groups/123/transfer",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ target_user_id: 456 }),
        }),
      );
    });

    it("throws error with backend message on failure", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ error: "Gruppe bereits übertragen" }),
      } as Response);

      await expect(
        groupTransferService.transferGroup("123", "456"),
      ).rejects.toThrow("Gruppe bereits übertragen");
    });

    it("throws default error when no backend message", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({}),
      } as Response);

      await expect(
        groupTransferService.transferGroup("123", "456"),
      ).rejects.toThrow("Transfer fehlgeschlagen");
    });
  });

  describe("getActiveTransfersForGroup", () => {
    it("returns mapped transfers from wrapped response", async () => {
      const backendData = [
        {
          id: 1,
          group_id: 100,
          regular_staff_id: null, // This is a transfer (no regular staff)
          substitute_staff_id: 50,
          substitute_staff: {
            person: {
              first_name: "Lisa",
              last_name: "Müller",
            },
          },
          start_date: "2024-01-01",
          end_date: "2024-01-02",
        },
      ];

      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: backendData }),
      } as Response);

      const result =
        await groupTransferService.getActiveTransfersForGroup("100");

      expect(result).toHaveLength(1);
      expect(result[0]).toEqual<GroupTransfer>({
        substitutionId: "1",
        groupId: "100",
        targetStaffId: "50",
        targetName: "Lisa Müller",
        validUntil: "2024-01-02",
      });
    });

    it("returns mapped transfers from direct array response", async () => {
      const backendData = [
        {
          id: 2,
          group_id: 200,
          regular_staff_id: null,
          substitute_staff_id: 60,
          substitute_staff: {
            person: {
              first_name: "Peter",
              last_name: "Schmidt",
            },
          },
          start_date: "2024-01-01",
          end_date: "2024-01-03",
        },
      ];

      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(backendData),
      } as Response);

      const result =
        await groupTransferService.getActiveTransfersForGroup("200");

      expect(result).toHaveLength(1);
      expect(result[0]?.targetName).toBe("Peter Schmidt");
    });

    it("filters out regular substitutions (has regular_staff_id)", async () => {
      const backendData = [
        {
          id: 1,
          group_id: 100,
          regular_staff_id: 30, // Not a transfer - has regular staff
          substitute_staff_id: 50,
          start_date: "2024-01-01",
          end_date: "2024-01-02",
        },
        {
          id: 2,
          group_id: 100,
          regular_staff_id: null, // This is a transfer
          substitute_staff_id: 60,
          substitute_staff: {
            person: {
              first_name: "Anna",
              last_name: "Test",
            },
          },
          start_date: "2024-01-01",
          end_date: "2024-01-02",
        },
      ];

      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: backendData }),
      } as Response);

      const result =
        await groupTransferService.getActiveTransfersForGroup("100");

      expect(result).toHaveLength(1);
      expect(result[0]?.targetName).toBe("Anna Test");
    });

    it("returns empty array on fetch error", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 404,
      } as Response);

      const result =
        await groupTransferService.getActiveTransfersForGroup("999");

      expect(result).toEqual([]);
    });

    it("uses provided token instead of getSession", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
      } as Response);

      await groupTransferService.getActiveTransfersForGroup(
        "100",
        "custom-token",
      );

      expect(fetchMock).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer custom-token",
          }) as HeadersInit,
        }),
      );
    });

    it("returns Unbekannt when substitute_staff.person is missing", async () => {
      const backendData = [
        {
          id: 1,
          group_id: 100,
          regular_staff_id: null,
          substitute_staff_id: 50,
          // No substitute_staff.person
          start_date: "2024-01-01",
          end_date: "2024-01-02",
        },
      ];

      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: backendData }),
      } as Response);

      const result =
        await groupTransferService.getActiveTransfersForGroup("100");

      expect(result[0]?.targetName).toBe("Unbekannt");
    });
  });

  describe("cancelTransferBySubstitutionId", () => {
    it("completes successfully", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ success: true }),
      } as Response);

      await expect(
        groupTransferService.cancelTransferBySubstitutionId("100", "1"),
      ).resolves.toBeUndefined();

      expect(fetchMock).toHaveBeenCalledWith(
        "/api/groups/100/transfer/1",
        expect.objectContaining({
          method: "DELETE",
        }),
      );
    });

    it("throws error with backend message on failure", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({ error: "Nicht berechtigt" }),
      } as Response);

      await expect(
        groupTransferService.cancelTransferBySubstitutionId("100", "1"),
      ).rejects.toThrow("Nicht berechtigt");
    });

    it("throws default error when no backend message", async () => {
      fetchMock.mockResolvedValueOnce({
        ok: false,
        json: () => Promise.resolve({}),
      } as Response);

      await expect(
        groupTransferService.cancelTransferBySubstitutionId("100", "1"),
      ).rejects.toThrow("Löschen fehlgeschlagen");
    });
  });
});
