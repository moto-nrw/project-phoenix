import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type {
  InvitationAcceptRequest,
  CreateInvitationRequest,
  BackendInvitationValidation,
  BackendInvitation,
} from "./invitation-helpers";
import {
  validateInvitation,
  acceptInvitation,
  createInvitation,
  listPendingInvitations,
  resendInvitation,
  revokeInvitation,
} from "./invitation-api";

// Mock the mapping functions
vi.mock("./invitation-helpers", async () => {
  const actual =
    // eslint-disable-next-line @typescript-eslint/consistent-type-imports
    await vi.importActual<typeof import("./invitation-helpers")>(
      "./invitation-helpers",
    );
  return {
    ...actual,
    mapInvitationValidationResponse: vi.fn(
      (data: BackendInvitationValidation) => ({
        email: data.email,
        roleName: data.role_name,
        firstName: data.first_name,
        lastName: data.last_name,
        position: data.position,
        expiresAt: data.expires_at,
      }),
    ),
    mapPendingInvitationResponse: vi.fn((data: BackendInvitation) => ({
      id: data.id,
      email: data.email,
      roleId: data.role_id,
      roleName: data.role_name ?? "",
      createdBy: data.created_by,
      creatorEmail: data.creator,
      expiresAt: data.expires_at,
      token: data.token,
      firstName: data.first_name,
      lastName: data.last_name,
      position: data.position,
    })),
  };
});

// Sample test data
const sampleBackendValidation: BackendInvitationValidation = {
  email: "teacher@example.com",
  role_name: "Teacher",
  first_name: "John",
  last_name: "Doe",
  position: "Math Teacher",
  expires_at: "2025-12-31T23:59:59Z",
};

const sampleBackendInvitation: BackendInvitation = {
  id: 1,
  email: "teacher@example.com",
  role_id: 2,
  role_name: "Teacher",
  token: "invitation-token-123",
  expires_at: "2025-12-31T23:59:59Z",
  created_by: 1,
  first_name: "John",
  last_name: "Doe",
  position: "Math Teacher",
  creator: "admin@example.com",
};

describe("invitation-api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Mock fetch globally
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("validateInvitation", () => {
    it("validates invitation successfully with direct data response", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => sampleBackendValidation,
      });

      const result = await validateInvitation("test-token-123");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/invitations/validate?token=test-token-123",
      );
      expect(result).toEqual({
        email: "teacher@example.com",
        roleName: "Teacher",
        firstName: "John",
        lastName: "Doe",
        position: "Math Teacher",
        expiresAt: "2025-12-31T23:59:59Z",
      });
    });

    it("validates invitation successfully with wrapped data response", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: sampleBackendValidation }),
      });

      const result = await validateInvitation("test-token-123");

      expect(result.email).toBe("teacher@example.com");
      expect(result.roleName).toBe("Teacher");
    });

    it("handles URL encoding of token", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => sampleBackendValidation,
      });

      await validateInvitation("token with spaces & special=chars");

      expect(global.fetch).toHaveBeenCalledWith(
        "/api/invitations/validate?token=token%20with%20spaces%20%26%20special%3Dchars",
      );
    });

    it("throws error with JSON error message when validation fails", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 404,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ error: "Invitation not found" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(validateInvitation("invalid-token")).rejects.toThrow(
        "Invitation not found",
      );
    });

    it("throws error with text error message when validation fails", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 410,
        headers: new Headers({ "Content-Type": "text/plain" }),
        text: async () => "Invitation expired",
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(validateInvitation("expired-token")).rejects.toThrow(
        "Invitation expired",
      );
    });

    it("uses fallback message when response body is empty", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({ "Content-Type": "text/plain" }),
        text: async () => "   ",
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(validateInvitation("test-token")).rejects.toThrow(
        "Einladung konnte nicht geprüft werden.",
      );
    });

    it("includes retryAfterSeconds from Retry-After header (numeric)", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": "60",
        }),
        json: async () => ({ error: "Too many requests" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      try {
        await validateInvitation("test-token");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as {
          status?: number;
          retryAfterSeconds?: number;
        };
        expect(apiError.status).toBe(429);
        expect(apiError.retryAfterSeconds).toBe(60);
      }
    });

    it("includes retryAfterSeconds from Retry-After header (date)", async () => {
      const futureDate = new Date(Date.now() + 120000); // 2 minutes from now

      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers({
          "Content-Type": "application/json",
          "Retry-After": futureDate.toUTCString(),
        }),
        json: async () => ({ error: "Rate limited" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      try {
        await validateInvitation("test-token");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as { retryAfterSeconds?: number };
        expect(apiError.retryAfterSeconds).toBeGreaterThan(0);
        expect(apiError.retryAfterSeconds).toBeLessThanOrEqual(121);
      }
    });

    it("handles negative Retry-After value (clamps to 0)", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers({
          "Retry-After": "-10",
        }),
        json: async () => ({ error: "Error" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      try {
        await validateInvitation("test-token");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as { retryAfterSeconds?: number };
        expect(apiError.retryAfterSeconds).toBe(0);
      }
    });

    it("handles past date Retry-After value (returns 0)", async () => {
      const pastDate = new Date(Date.now() - 30000);

      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers({
          "Retry-After": pastDate.toUTCString(),
        }),
        json: async () => ({ error: "Error" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      try {
        await validateInvitation("test-token");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as { retryAfterSeconds?: number };
        expect(apiError.retryAfterSeconds).toBe(0);
      }
    });

    it("handles invalid Retry-After value", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers({
          "Retry-After": "not-a-number-or-date",
        }),
        json: async () => ({ error: "Error" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      try {
        await validateInvitation("test-token");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as { retryAfterSeconds?: number };
        expect(apiError.retryAfterSeconds).toBeUndefined();
      }
    });

    it("handles error response parse failure gracefully", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => {
          throw new Error("JSON parse error");
        },
        text: async () => {
          throw new Error("Text parse error");
        },
      });

      const warnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => undefined);

      await expect(validateInvitation("test-token")).rejects.toThrow(
        "Einladung konnte nicht geprüft werden.",
      );
      expect(warnSpy).toHaveBeenCalledWith(
        "Failed to parse invitation API error",
        expect.any(Error),
      );
    });

    it("extracts message field from JSON error response", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 400,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ message: "Invalid token format" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(validateInvitation("test-token")).rejects.toThrow(
        "Invalid token format",
      );
    });
  });

  describe("acceptInvitation", () => {
    const acceptRequest: InvitationAcceptRequest = {
      firstName: "John",
      lastName: "Doe",
      password: "SecurePass123!",
      confirmPassword: "SecurePass123!",
    };

    it("accepts invitation successfully", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "success" }),
      });

      await acceptInvitation("test-token-123", acceptRequest);

      expect(global.fetch).toHaveBeenCalledWith("/api/invitations/accept", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          token: "test-token-123",
          ...acceptRequest,
        }),
      });
    });

    it("throws error when accept fails", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 409,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ error: "Account already exists" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(
        acceptInvitation("test-token", acceptRequest),
      ).rejects.toThrow("Account already exists");
    });

    it("uses fallback error message when response is empty", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({}),
        json: async () => ({}),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(
        acceptInvitation("test-token", acceptRequest),
      ).rejects.toThrow("Einladung konnte nicht angenommen werden.");
    });
  });

  describe("createInvitation", () => {
    const createRequest: CreateInvitationRequest = {
      email: "newteacher@example.com",
      roleId: 2,
      firstName: "Jane",
      lastName: "Smith",
      position: "Science Teacher",
    };

    it("creates invitation successfully with direct data response", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => sampleBackendInvitation,
      });

      const result = await createInvitation(createRequest);

      expect(global.fetch).toHaveBeenCalledWith("/api/invitations", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(createRequest),
        credentials: "include",
      });
      expect(result.email).toBe("teacher@example.com");
      expect(result.roleId).toBe(2);
    });

    it("creates invitation successfully with wrapped data response", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: sampleBackendInvitation }),
      });

      const result = await createInvitation(createRequest);

      expect(result.id).toBe(1);
      expect(result.token).toBe("invitation-token-123");
    });

    it("throws error when email already invited", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 409,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ error: "Email already has pending invitation" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(createInvitation(createRequest)).rejects.toThrow(
        "Email already has pending invitation",
      );
    });

    it("uses fallback error message on failure", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({}),
        json: async () => ({}),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(createInvitation(createRequest)).rejects.toThrow(
        "Einladung konnte nicht erstellt werden.",
      );
    });
  });

  describe("listPendingInvitations", () => {
    it("lists invitations successfully with array response", async () => {
      const invitations = [sampleBackendInvitation];

      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: invitations }),
      });

      const result = await listPendingInvitations();

      expect(global.fetch).toHaveBeenCalledWith("/api/invitations", {
        credentials: "include",
      });
      expect(result).toHaveLength(1);
      expect(result[0]?.email).toBe("teacher@example.com");
    });

    it("lists invitations with direct data array", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => [sampleBackendInvitation],
      });

      const result = await listPendingInvitations();

      expect(result).toHaveLength(1);
    });

    it("handles single invitation response as array", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: sampleBackendInvitation }),
      });

      const result = await listPendingInvitations();

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe(1);
    });

    it("throws error when fetch fails", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 403,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ error: "Forbidden" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(listPendingInvitations()).rejects.toThrow("Forbidden");
    });

    it("uses fallback error message on failure", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({}),
        json: async () => ({}),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(listPendingInvitations()).rejects.toThrow(
        "Offene Einladungen konnten nicht geladen werden.",
      );
    });
  });

  describe("resendInvitation", () => {
    it("resends invitation successfully", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "success" }),
      });

      await resendInvitation(123);

      expect(global.fetch).toHaveBeenCalledWith("/api/invitations/123/resend", {
        method: "POST",
        credentials: "include",
      });
    });

    it("throws error when resend fails", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 404,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ error: "Invitation not found" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(resendInvitation(999)).rejects.toThrow(
        "Invitation not found",
      );
    });

    it("uses fallback error message on failure", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({}),
        json: async () => ({}),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(resendInvitation(123)).rejects.toThrow(
        "Einladung konnte nicht erneut gesendet werden.",
      );
    });
  });

  describe("revokeInvitation", () => {
    it("revokes invitation successfully", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "success" }),
      });

      await revokeInvitation(123);

      expect(global.fetch).toHaveBeenCalledWith("/api/invitations/123", {
        method: "DELETE",
        credentials: "include",
      });
    });

    it("throws error when revoke fails", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 404,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ error: "Invitation not found" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(revokeInvitation(999)).rejects.toThrow(
        "Invitation not found",
      );
    });

    it("uses fallback error message on failure", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({}),
        json: async () => ({}),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(revokeInvitation(123)).rejects.toThrow(
        "Einladung konnte nicht widerrufen werden.",
      );
    });
  });

  describe("edge cases and error handling", () => {
    it("handles non-JSON content type with empty body", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500,
        headers: new Headers({ "Content-Type": "text/html" }),
        text: async () => "",
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(validateInvitation("test-token")).rejects.toThrow(
        "Einladung konnte nicht geprüft werden.",
      );
    });

    it("handles response with both error and message fields (error takes precedence)", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 400,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({
          error: "Primary error message",
          message: "Secondary message",
        }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      await expect(validateInvitation("test-token")).rejects.toThrow(
        "Primary error message",
      );
    });

    it("extracts data from single-level wrapper only", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: sampleBackendValidation,
        }),
      });

      const result = await validateInvitation("test-token");

      // Should successfully extract single-level data wrapper
      expect(result.email).toBe("teacher@example.com");
      expect(result.roleName).toBe("Teacher");
    });

    it("preserves original response status in error", async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 418,
        headers: new Headers({ "Content-Type": "application/json" }),
        json: async () => ({ error: "I'm a teapot" }),
      });

      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      try {
        await validateInvitation("test-token");
        expect.fail("Should have thrown");
      } catch (error) {
        const apiError = error as { status?: number };
        expect(apiError.status).toBe(418);
      }
    });
  });
});
