import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import {
  listRelatedAccounts,
  inviteRelatedAccount,
  removeRelatedAccount,
  type RelatedAccount,
} from "./parent-api";

const originalFetch = globalThis.fetch;

function mockFetch(
  impl: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>,
) {
  globalThis.fetch = vi.fn(impl) as typeof globalThis.fetch;
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

beforeEach(() => {
  globalThis.fetch = originalFetch;
});

afterEach(() => {
  globalThis.fetch = originalFetch;
});

const sampleAccount: RelatedAccount = {
  guardian_profile_id: "10",
  first_name: "Sabine",
  last_name: "Schneider",
  email: "sabine@x.de",
  relationship_type: "parent",
  is_primary: true,
  status: "active",
  is_self: false,
};

const noAccountSample: RelatedAccount = {
  ...sampleAccount,
  guardian_profile_id: "11",
  status: "no_account",
};

describe("parent-api related accounts", () => {
  describe("listRelatedAccounts", () => {
    it("returns the unwrapped account list", async () => {
      let calledUrl = "";
      mockFetch(async (input) => {
        calledUrl = String(input);
        return jsonResponse({ data: [sampleAccount] });
      });
      const out = await listRelatedAccounts("3");
      expect(out).toEqual([sampleAccount]);
      expect(calledUrl).toBe("/api/parent/me/children/3/related-accounts");
    });

    it("accepts no-account contact rows from the backend", async () => {
      mockFetch(async () => jsonResponse({ data: [noAccountSample] }));
      const out = await listRelatedAccounts("3");
      expect(out[0]?.status).toBe("no_account");
    });

    it("throws on a non-ok response", async () => {
      mockFetch(async () => jsonResponse({ error: "boom" }, { status: 500 }));
      await expect(listRelatedAccounts("3")).rejects.toThrow("boom");
    });
  });

  describe("inviteRelatedAccount", () => {
    it("posts email + names and returns the outcome", async () => {
      let body: unknown;
      mockFetch(async (_input, init) => {
        body = JSON.parse(String(init?.body));
        return jsonResponse({
          data: { outcome: "invited", guardian_profile_id: "11" },
        });
      });
      const out = await inviteRelatedAccount("3", "oma@x.de", {
        firstName: "Oma",
        lastName: "Wolf",
      });
      expect(out.outcome).toBe("invited");
      expect(body).toEqual({
        email: "oma@x.de",
        first_name: "Oma",
        last_name: "Wolf",
      });
    });

    it("defaults missing names to empty strings", async () => {
      let body: unknown;
      mockFetch(async (_input, init) => {
        body = JSON.parse(String(init?.body));
        return jsonResponse({ data: { outcome: "pending_approval" } });
      });
      await inviteRelatedAccount("3", "oma@x.de");
      expect(body).toEqual({
        email: "oma@x.de",
        first_name: "",
        last_name: "",
      });
    });

    it("surfaces the backend error on failure", async () => {
      mockFetch(async () =>
        jsonResponse({ error: "Einladen deaktiviert" }, { status: 403 }),
      );
      await expect(inviteRelatedAccount("3", "x@x.de")).rejects.toThrow(
        "Einladen deaktiviert",
      );
    });

    it("sends confirm_role_upgrade only when confirming an upgrade", async () => {
      let body: unknown;
      mockFetch(async (_input, init) => {
        body = JSON.parse(String(init?.body));
        return jsonResponse({
          data: { outcome: "already_linked", guardian_profile_id: "11" },
        });
      });
      await inviteRelatedAccount("3", "opa@x.de", { confirmRoleUpgrade: true });
      expect(body).toEqual({
        email: "opa@x.de",
        first_name: "",
        last_name: "",
        confirm_role_upgrade: true,
      });
    });

    it("returns existing_role for the restricted-contact outcome", async () => {
      mockFetch(async () =>
        jsonResponse({
          data: {
            outcome: "existing_contact_restricted",
            guardian_profile_id: "11",
            existing_role: "emergency_contact",
          },
        }),
      );
      const out = await inviteRelatedAccount("3", "opa@x.de");
      expect(out.outcome).toBe("existing_contact_restricted");
      expect(out.existing_role).toBe("emergency_contact");
    });
  });

  describe("removeRelatedAccount", () => {
    it("resolves on a 200 DELETE", async () => {
      let method = "";
      let url = "";
      mockFetch(async (input, init) => {
        url = String(input);
        method = String(init?.method);
        return new Response("", { status: 200 });
      });
      await expect(removeRelatedAccount("3", "10")).resolves.toBeUndefined();
      expect(method).toBe("DELETE");
      expect(url).toBe("/api/parent/me/children/3/related-accounts/10");
    });

    it("throws the backend error on a non-ok response", async () => {
      mockFetch(async () =>
        jsonResponse({ error: "Primär geschützt" }, { status: 403 }),
      );
      await expect(removeRelatedAccount("3", "10")).rejects.toThrow(
        "Primär geschützt",
      );
    });

    it("falls back to a generic message when the error body is unshaped", async () => {
      mockFetch(async () => new Response("not json", { status: 500 }));
      await expect(removeRelatedAccount("3", "10")).rejects.toThrow(
        /Request failed \(500\)/,
      );
    });

    it("redirects to the parents login on 401", async () => {
      const assign = vi.fn();
      Object.defineProperty(window, "location", {
        writable: true,
        value: { assign, host: "parents.localhost:3000" },
      });
      mockFetch(async () => new Response("", { status: 401 }));
      await expect(removeRelatedAccount("3", "10")).rejects.toThrow();
      expect(assign).toHaveBeenCalledWith("/parents/login");
    });
  });
});
