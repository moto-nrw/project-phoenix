import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getPhaseResponseOverview,
  mapPhaseResponseOverview,
} from "./enrollment-phase-api";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("mapPhaseResponseOverview", () => {
  it("turns numeric ids into strings and keeps absent links null", () => {
    const overview = mapPhaseResponseOverview({
      applicable: true,
      expected: 2,
      responded: 1,
      children: [
        {
          student_id: 7,
          first_name: "Mia",
          last_name: "Arslan",
          school_class: "2a",
          has_parent_app: false,
          responded: false,
          pending_request_id: 901,
          child_status: "pending_renewal",
        },
        {
          student_id: 8,
          first_name: "Ben",
          last_name: "Yilmaz",
          school_class: "1b",
          has_parent_app: true,
          responded: true,
          request_id: 900,
          child_status: "submitted",
        },
      ],
      excluded: [{ reason: "graduating", count: 3 }],
    });

    expect(overview.children[0]).toEqual({
      studentId: "7",
      firstName: "Mia",
      lastName: "Arslan",
      schoolClass: "2a",
      hasParentApp: false,
      responded: false,
      requestId: null,
      pendingRequestId: "901",
      childStatus: "pending_renewal",
    });
    expect(overview.children[1]?.requestId).toBe("900");
    expect(overview.children[1]?.pendingRequestId).toBeNull();
    expect(overview.excluded).toEqual([{ reason: "graduating", count: 3 }]);
  });

  it("treats missing lists as empty", () => {
    const overview = mapPhaseResponseOverview({
      applicable: false,
      expected: 0,
      responded: 0,
      children: null,
    });
    expect(overview.children).toEqual([]);
    expect(overview.excluded).toEqual([]);
  });
});

describe("getPhaseResponseOverview", () => {
  it("reads the enveloped payload of the phase", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "success",
          data: { applicable: true, expected: 1, responded: 0, children: [] },
        }),
        { status: 200 },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const overview = await getPhaseResponseOverview("12");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/enrollment/phases/12/responses",
      { cache: "no-store" },
    );
    expect(overview.applicable).toBe(true);
    expect(overview.expected).toBe(1);
  });

  it("throws a readable error when the request fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("{}", { status: 500 })),
    );
    await expect(getPhaseResponseOverview("12")).rejects.toThrow();
  });
});
