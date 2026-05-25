import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import {
  listCareOfferings,
  createCareOffering,
  updateCareOffering,
  deleteCareOffering,
  cloneCareOffering,
  type CareOffering,
  type CareOfferingInput,
} from "./care-offering-api";

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

function mkOffering(id: string): CareOffering {
  return {
    id,
    phase_id: "5678",
    name: "OGS",
    days_of_week_mode: "fixed",
    available_days: ["mon", "tue", "wed", "thu", "fri"],
    includes_holiday_care: false,
    includes_lunch: false,
    is_active: true,
    sort_order: 0,
    created_at: "2026-04-01T12:00:00Z",
    updated_at: "2026-04-01T12:00:00Z",
  };
}

const validInput: CareOfferingInput = {
  phase_id: 5678,
  name: "OGS",
  days_of_week_mode: "fixed",
  available_days: ["mon", "tue"],
  includes_holiday_care: false,
  includes_lunch: false,
  is_active: true,
  sort_order: 0,
};

// --- listCareOfferings ----------------------------------------------

describe("listCareOfferings", () => {
  it("returns the unwrapped array on 200 with no filter", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [mkOffering("1234")] });
    });
    const out = await listCareOfferings();
    expect(out).toHaveLength(1);
    expect(out[0]!.id).toBe("1234");
    expect(seenURL).not.toContain("phase_id");
  });

  it("appends phase_id query param when provided", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: [] });
    });
    await listCareOfferings("42");
    expect(seenURL).toContain("phase_id=42");
  });

  it("returns [] when backend returns non-array data", async () => {
    mockFetch(async () => jsonResponse({ data: null }));
    expect(await listCareOfferings()).toEqual([]);
  });

  it("throws with backend error message on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "permission denied" }, { status: 403 }),
    );
    await expect(listCareOfferings()).rejects.toThrow(/permission denied/);
  });

  it("appends HTTP status when body has no error", async () => {
    mockFetch(async () => jsonResponse({}, { status: 500 }));
    await expect(listCareOfferings()).rejects.toThrow(/HTTP 500/);
  });
});

// --- createCareOffering --------------------------------------------

describe("createCareOffering", () => {
  it("POSTs the body and returns the unwrapped offering", async () => {
    let seenBody = "";
    mockFetch(async (_, init) => {
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({ data: mkOffering("1234") }, { status: 201 });
    });
    const out = await createCareOffering(validInput);
    expect(out.id).toBe("1234");
    expect(seenBody).toContain(`"phase_id":5678`);
    expect(seenBody).toContain(`"name":"OGS"`);
  });

  it("throws on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "validation failed" }, { status: 400 }),
    );
    await expect(createCareOffering(validInput)).rejects.toThrow(
      /validation failed/,
    );
  });
});

// --- updateCareOffering --------------------------------------------

describe("updateCareOffering", () => {
  it("PUTs to the id-bearing URL and returns the unwrapped offering", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkOffering("1234") });
    });
    const out = await updateCareOffering("1234", validInput);
    expect(out.id).toBe("1234");
    expect(seenURL).toContain("/1234");
  });

  it("URL-encodes the id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkOffering("a/b") });
    });
    await updateCareOffering("a/b", validInput);
    expect(seenURL).toContain("a%2Fb");
  });

  it("throws on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "service unavailable" }, { status: 500 }),
    );
    await expect(updateCareOffering("1234", validInput)).rejects.toThrow(
      /service unavailable/,
    );
  });
});

// --- deleteCareOffering --------------------------------------------

describe("deleteCareOffering", () => {
  it("resolves on 204", async () => {
    mockFetch(async () => new Response(null, { status: 204 }));
    await expect(deleteCareOffering("1234")).resolves.toBeUndefined();
  });

  it("URL-encodes the id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return new Response(null, { status: 204 });
    });
    await deleteCareOffering("a/b");
    expect(seenURL).toContain("a%2Fb");
  });

  it("throws with backend error on non-OK non-204", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "still referenced" }, { status: 409 }),
    );
    await expect(deleteCareOffering("1234")).rejects.toThrow(
      /still referenced/,
    );
  });
});

// --- cloneCareOffering ---------------------------------------------

describe("cloneCareOffering", () => {
  it("POSTs to /clone with target_phase_id and returns the unwrapped clone", async () => {
    let seenURL = "";
    let seenBody = "";
    mockFetch(async (input, init) => {
      seenURL = typeof input === "string" ? input : input.toString();
      seenBody = (init?.body as string) ?? "";
      return jsonResponse({ data: mkOffering("9999") }, { status: 201 });
    });
    const out = await cloneCareOffering("1234", { target_phase_id: 5678 });
    expect(out.id).toBe("9999");
    expect(seenURL).toContain("/1234/clone");
    expect(seenBody).toContain(`"target_phase_id":5678`);
  });

  it("URL-encodes the source id", async () => {
    let seenURL = "";
    mockFetch(async (input) => {
      seenURL = typeof input === "string" ? input : input.toString();
      return jsonResponse({ data: mkOffering("1") }, { status: 201 });
    });
    await cloneCareOffering("a/b", { target_phase_id: 5 });
    expect(seenURL).toContain("a%2Fb/clone");
  });

  it("throws on non-OK", async () => {
    mockFetch(async () =>
      jsonResponse({ error: "source not found" }, { status: 404 }),
    );
    await expect(
      cloneCareOffering("1234", { target_phase_id: 5 }),
    ).rejects.toThrow(/source not found/);
  });
});
