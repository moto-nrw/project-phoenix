import { describe, it, expect, vi, beforeEach } from "vitest";
import type {
  BackendInvitationsListResponse,
  BackendInvitationValidation,
} from "./operator-invitation-helpers";

// Hoist mocks
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

// Must import after mocks
import {
  createOperatorInvitation,
  listOperatorInvitations,
  resendOperatorInvitation,
  revokeOperatorInvitation,
  establishOperatorInvitationSession,
  validateOperatorInvitation,
  acceptOperatorInvitation,
} from "./operator-invitation-api";

describe("operator invitation api", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("createOperatorInvitation", () => {
    it("converts camelCase frontend shape to snake_case backend body", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await createOperatorInvitation({
        email: "new@example.com",
        displayName: "New Op",
      });

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/invitations",
        {
          method: "POST",
          body: { email: "new@example.com", display_name: "New Op" },
        },
      );
    });
  });

  describe("listInvitations", () => {
    it("returns mapped invitations and operators", async () => {
      const raw: BackendInvitationsListResponse = {
        invitations: [
          {
            id: 1,
            email: "invited@example.com",
            display_name: "Test",
            created_by: 2,
            creator_name: "Admin",
            expires_at: "2026-04-06T00:00:00Z",
            email_sent_at: "2026-04-04T00:00:00Z",
            email_error: null,
            email_retry_count: 0,
            created_at: "2026-04-04T00:00:00Z",
          },
        ],
        operators: [
          {
            id: 2,
            email: "admin@example.com",
            display_name: "Admin",
            active: true,
            last_login: "2026-04-03T00:00:00Z",
            created_at: "2026-01-01T00:00:00Z",
          },
        ],
      };
      mockOperatorFetch.mockResolvedValue(raw);

      const result = await listOperatorInvitations();

      expect(result.invitations).toHaveLength(1);
      expect(result.invitations[0]!.id).toBe("1");
      expect(result.invitations[0]!.email).toBe("invited@example.com");
      expect(result.operators).toHaveLength(1);
      expect(result.operators[0]!.id).toBe("2");
    });

    it("handles null invitations and operators arrays", async () => {
      mockOperatorFetch.mockResolvedValue({
        invitations: null,
        operators: null,
      });

      const result = await listOperatorInvitations();

      expect(result.invitations).toEqual([]);
      expect(result.operators).toEqual([]);
    });
  });

  describe("resendOperatorInvitation", () => {
    it("sends POST to resend endpoint with encoded ID", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await resendOperatorInvitation("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/invitations/42/resend",
        { method: "POST" },
      );
    });
  });

  describe("revokeOperatorInvitation", () => {
    it("sends DELETE to invitation endpoint", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await revokeOperatorInvitation("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/invitations/42",
        { method: "DELETE" },
      );
    });
  });
});

describe("public invitation session", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("moves the URL token into the server-owned session", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ flow_id: "flow-1" }),
    });

    await expect(
      establishOperatorInvitationSession("valid-token"),
    ).resolves.toBe("flow-1");

    expect(global.fetch).toHaveBeenCalledWith(
      "/api/operator/auth/invitations/session",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ token: "valid-token" }),
      }),
    );
  });
});

describe("validateOperatorInvitation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns mapped validation data on success", async () => {
    const backendData: BackendInvitationValidation = {
      email: "invited@example.com",
      display_name: "Test User",
      expires_at: "2026-04-06T00:00:00Z",
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        data: backendData,
      }),
    });

    const result = await validateOperatorInvitation("flow-1");

    expect(result.email).toBe("invited@example.com");
    expect(result.displayName).toBe("Test User");
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/operator/auth/invitations/validate",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "x-operator-invitation-flow": "flow-1",
        }),
        body: JSON.stringify({}),
      }),
    );
  });

  it("throws with backend message on error", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: async () => ({
        message: "Dieser Link ist abgelaufen oder ungültig",
      }),
    });

    await expect(validateOperatorInvitation("flow-1")).rejects.toThrow(
      "Dieser Link ist abgelaufen oder ungültig",
    );
  });

  it("throws default message when error JSON has no message", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => ({}),
    });

    await expect(validateOperatorInvitation("flow-1")).rejects.toThrow(
      "Einladung nicht gefunden oder abgelaufen",
    );
  });

  it("throws default message when error response is not JSON", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => {
        throw new Error("not JSON");
      },
    });

    await expect(validateOperatorInvitation("flow-1")).rejects.toThrow(
      "Einladung nicht gefunden oder abgelaufen",
    );
  });

  it("handles unwrapped response (no envelope)", async () => {
    const backendData: BackendInvitationValidation = {
      email: "direct@example.com",
      display_name: null,
      expires_at: "2026-04-06T00:00:00Z",
    };

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => backendData,
    });

    const result = await validateOperatorInvitation("flow-1");
    expect(result.email).toBe("direct@example.com");
    expect(result.displayName).toBeUndefined();
  });
});

describe("acceptOperatorInvitation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends POST with form data while the server supplies the token", async () => {
    global.fetch = vi.fn().mockResolvedValue({ ok: true });

    await acceptOperatorInvitation("flow-1", {
      displayName: "New Op",
      password: "Str0ng!Pass",
      confirmPassword: "Str0ng!Pass",
    });

    expect(global.fetch).toHaveBeenCalledWith(
      "/api/operator/auth/invitations/accept",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "x-operator-invitation-flow": "flow-1",
        }),
        body: JSON.stringify({
          display_name: "New Op",
          password: "Str0ng!Pass",
          confirm_password: "Str0ng!Pass",
        }),
      }),
    );
  });

  it("throws with backend message on error", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: async () => ({ message: "Passwort zu schwach" }),
    });

    await expect(
      acceptOperatorInvitation("flow-1", {
        displayName: "Test",
        password: "weak",
        confirmPassword: "weak",
      }),
    ).rejects.toThrow("Passwort zu schwach");
  });

  it("throws default message on non-JSON error", async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: async () => {
        throw new Error("not JSON");
      },
    });

    await expect(
      acceptOperatorInvitation("flow-1", {
        displayName: "Test",
        password: "pass",
        confirmPassword: "pass",
      }),
    ).rejects.toThrow("Einladung konnte nicht angenommen werden");
  });
});
