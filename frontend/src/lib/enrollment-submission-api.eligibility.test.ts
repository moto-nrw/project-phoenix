import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchParentEnrollmentBootstrap,
  fetchPublicCareOfferings,
} from "./enrollment-submission-api";

const mockFetch = vi.fn<typeof fetch>();
const originalFetch = global.fetch;

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

// Covers the per-phase eligibility additions (#1663) on the client: the
// grade-restriction normalizer, and the parent-scoped bootstrap that exists
// precisely because the public endpoint refuses audience-restricted phases.
describe("enrollment eligibility client", () => {
  beforeEach(() => {
    mockFetch.mockReset();
    global.fetch = mockFetch;
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  describe("grade-restriction normalization", () => {
    it("keeps positive integer grades from the payload", async () => {
      mockFetch.mockResolvedValueOnce(
        jsonResponse({ offerings: [], eligible_grade_levels: [1, 3] }),
      );

      const result = await fetchPublicCareOfferings("demo-school", "7");

      expect(result.eligibleGradeLevels).toEqual([1, 3]);
    });

    it("drops entries that cannot be a grade", async () => {
      // An untrusted payload must not put a nonsense value in front of the
      // parent as a selectable grade.
      mockFetch.mockResolvedValueOnce(
        jsonResponse({
          offerings: [],
          eligible_grade_levels: [2, "3", 0, -1, 1.5, null, 4],
        }),
      );

      const result = await fetchPublicCareOfferings("demo-school", "7");

      expect(result.eligibleGradeLevels).toEqual([2, 4]);
    });

    it("reads a non-array restriction as no restriction", async () => {
      // A stale backend sending the wrong shape must degrade to "open to every
      // grade", never to "no grade is selectable".
      mockFetch.mockResolvedValueOnce(
        jsonResponse({ offerings: [], eligible_grade_levels: "1,2" }),
      );

      const result = await fetchPublicCareOfferings("demo-school", "7");

      expect(result.eligibleGradeLevels).toEqual([]);
    });

    it("leaves the restriction undefined when the backend omits it", async () => {
      mockFetch.mockResolvedValueOnce(jsonResponse({ offerings: [] }));

      const result = await fetchPublicCareOfferings("demo-school", "7");

      expect(result.eligibleGradeLevels).toBeUndefined();
    });
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
