import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fetchParentEnrollmentBootstrap } from "./enrollment-submission-api";

const mockFetch = vi.fn<typeof fetch>();
const originalFetch = global.fetch;

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

// The parent-scoped bootstrap (#1663) exists because the public endpoint
// refuses audience-restricted phases.
describe("enrollment eligibility client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch;
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  describe("fetchParentEnrollmentBootstrap", () => {
    it("loads the form from the parent-scoped endpoint", async () => {
      mockFetch.mockResolvedValueOnce(jsonResponse({ phase: { id: "7" } }));

      const result = await fetchParentEnrollmentBootstrap("demo-school", "7");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/parent/enrollments/demo-school/bootstrap/7",
        { cache: "no-store" },
      );
      expect(result).toEqual({ phase: { id: "7" } });
    });

    it("passes a late-invite token through as a query parameter", async () => {
      mockFetch.mockResolvedValueOnce(jsonResponse({ phase: { id: "7" } }));

      await fetchParentEnrollmentBootstrap("demo-school", "7", {
        lateInviteToken: "tok en/1",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/parent/enrollments/demo-school/bootstrap/7?late_invite=tok%20en%2F1",
        { cache: "no-store" },
      );
    });

    it("encodes slug and phase id", async () => {
      mockFetch.mockResolvedValueOnce(jsonResponse({ phase: { id: "a/b" } }));

      await fetchParentEnrollmentBootstrap("de mo/school", "a/b");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/parent/enrollments/de%20mo%2Fschool/bootstrap/a%2Fb",
        { cache: "no-store" },
      );
    });

    it("surfaces a failed load as an error", async () => {
      mockFetch.mockResolvedValueOnce(
        jsonResponse({ error: "Phase nicht verfügbar" }, { status: 404 }),
      );

      await expect(
        fetchParentEnrollmentBootstrap("demo-school", "7"),
      ).rejects.toThrow();
    });
  });
});
