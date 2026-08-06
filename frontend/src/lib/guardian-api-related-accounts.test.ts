import { describe, it, expect, vi, afterEach } from "vitest";
import {
  inviteGuardianToStudent,
  listPendingApprovals,
  approveGuardianInvitation,
  rejectGuardianInvitation,
} from "./guardian-api";

interface Captured {
  url?: string;
  init?: RequestInit;
}

// mockFetch installs a fetch stub returning `body` (duck-typed Response) and
// records the request so tests can assert URL/method/body.
function mockFetch(body: unknown): Captured {
  const captured: Captured = {};
  global.fetch = vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
    captured.url = String(url);
    captured.init = init;
    return Promise.resolve(body as Response);
  }) as typeof global.fetch;
  return captured;
}

function okJson(body: unknown) {
  return { ok: true, status: 200, json: () => Promise.resolve(body) };
}
function errStatus(status: number, body: unknown) {
  return {
    ok: false,
    status,
    statusText: "Err",
    json: () => Promise.resolve(body),
  };
}

describe("guardian-api related accounts", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("inviteGuardianToStudent", () => {
    it("posts the email + options and returns the result", async () => {
      const cap = mockFetch(
        okJson({
          status: "success",
          data: {
            outcome: "invited",
            guardian_profile_id: "9",
            invitation_id: "42",
          },
        }),
      );

      const result = await inviteGuardianToStudent("5", "a@b.de", {
        firstName: "A",
        lastName: "B",
        relationshipType: "guardian",
      });

      expect(result.outcome).toBe("invited");
      expect(result.invitation_id).toBe("42");
      expect(cap.url).toBe("/api/guardians/students/5/invite");
      expect(cap.init?.method).toBe("POST");
      expect(JSON.parse(String(cap.init?.body))).toEqual({
        email: "a@b.de",
        first_name: "A",
        last_name: "B",
        relationship_type: "guardian",
      });
    });

    it("defaults optional fields to empty strings", async () => {
      const cap = mockFetch(
        okJson({
          status: "success",
          data: { outcome: "invited", guardian_profile_id: "9" },
        }),
      );
      await inviteGuardianToStudent("5", "a@b.de");
      expect(JSON.parse(String(cap.init?.body))).toEqual({
        email: "a@b.de",
        first_name: "",
        last_name: "",
        relationship_type: "",
      });
    });

    it("throws the backend error message on an error status", async () => {
      mockFetch(okJson({ status: "error", error: "boom" }));
      await expect(inviteGuardianToStudent("5", "a@b.de")).rejects.toThrow(
        "boom",
      );
    });

    it("throws when the success payload has no data", async () => {
      mockFetch(okJson({ status: "success" }));
      await expect(inviteGuardianToStudent("5", "a@b.de")).rejects.toThrow(
        "Failed to invite guardian",
      );
    });

    it("throws the error from a non-ok response body", async () => {
      mockFetch(errStatus(400, { error: "Ungültige E-Mail" }));
      await expect(inviteGuardianToStudent("5", "x")).rejects.toThrow(
        "Ungültige E-Mail",
      );
    });

    it("falls back to statusText when the error body is unshaped", async () => {
      mockFetch(errStatus(500, { unexpected: true }));
      await expect(inviteGuardianToStudent("5", "x")).rejects.toThrow(
        /Failed to invite guardian: Err/,
      );
    });

    it("handles a non-JSON error response", async () => {
      mockFetch({
        ok: false,
        status: 502,
        statusText: "Bad Gateway",
        json: () => Promise.reject(new Error("not json")),
      });
      await expect(inviteGuardianToStudent("5", "x")).rejects.toThrow(
        "Failed to invite guardian",
      );
    });

    it("sends confirm_role_upgrade only when confirming an upgrade", async () => {
      const cap = mockFetch(
        okJson({
          status: "success",
          data: { outcome: "already_linked", guardian_profile_id: "9" },
        }),
      );
      await inviteGuardianToStudent("5", "a@b.de", {
        confirmRoleUpgrade: true,
      });
      expect(JSON.parse(String(cap.init?.body))).toEqual({
        email: "a@b.de",
        first_name: "",
        last_name: "",
        relationship_type: "",
        confirm_role_upgrade: true,
      });
    });

    it("returns existing_role for the restricted-contact outcome", async () => {
      mockFetch(
        okJson({
          status: "success",
          data: {
            outcome: "existing_contact_restricted",
            guardian_profile_id: "9",
            existing_role: "pickup_only",
          },
        }),
      );
      const result = await inviteGuardianToStudent("5", "a@b.de");
      expect(result.outcome).toBe("existing_contact_restricted");
      expect(result.existing_role).toBe("pickup_only");
    });
  });

  describe("listPendingApprovals", () => {
    it("maps the backend rows to camelCase", async () => {
      const cap = mockFetch(
        okJson({
          status: "success",
          data: [
            {
              id: "1",
              guardian_profile_id: "10",
              guardian_name: "Julia",
              guardian_email: "j@x.de",
              student_id: "3",
              student_name: "Felix",
              requested_by_email: "k@x.de",
              created_at: "2026-06-10T08:00:00Z",
              expires_at: "2026-06-12T08:00:00Z",
            },
          ],
        }),
      );
      const result = await listPendingApprovals();
      expect(result).toHaveLength(1);
      expect(result[0]).toMatchObject({
        id: "1",
        guardianProfileId: "10",
        guardianName: "Julia",
        guardianEmail: "j@x.de",
        studentName: "Felix",
        requestedByEmail: "k@x.de",
      });
      expect(cap.url).toBe("/api/guardians/invitations/pending-approval");
    });

    it("returns an empty array when data is missing", async () => {
      mockFetch(okJson({ status: "success" }));
      expect(await listPendingApprovals()).toEqual([]);
    });

    it("throws on an error status", async () => {
      mockFetch(okJson({ status: "error", error: "nope" }));
      await expect(listPendingApprovals()).rejects.toThrow("nope");
    });

    it("throws on a non-ok response", async () => {
      mockFetch(errStatus(403, { error: "Keine Berechtigung" }));
      await expect(listPendingApprovals()).rejects.toThrow(
        "Keine Berechtigung",
      );
    });
  });

  describe("approve / reject invitation", () => {
    it("approves via POST and tolerates a 204", async () => {
      const cap = mockFetch({
        ok: true,
        status: 204,
        json: () => Promise.reject(new Error("no body")),
      });
      await expect(approveGuardianInvitation("7")).resolves.toBeUndefined();
      expect(cap.url).toBe("/api/guardians/invitations/7/approve");
      expect(cap.init?.method).toBe("POST");
    });

    it("rejects via POST and reads a JSON envelope", async () => {
      const cap = mockFetch(okJson({ status: "success", data: null }));
      await expect(rejectGuardianInvitation("7")).resolves.toBeUndefined();
      expect(cap.url).toBe("/api/guardians/invitations/7/reject");
      expect(cap.init?.method).toBe("POST");
    });

    it("throws when the action status is error", async () => {
      mockFetch(okJson({ status: "error", error: "cannot" }));
      await expect(approveGuardianInvitation("7")).rejects.toThrow("cannot");
    });

    it("throws when the action response is not ok", async () => {
      mockFetch(errStatus(404, { error: "not found" }));
      await expect(rejectGuardianInvitation("7")).rejects.toThrow("not found");
    });
  });
});
