import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./session-cache", () => ({ sessionFetch: vi.fn() }));

import { sessionFetch } from "./session-cache";
import { fetchHomeLayout } from "./home-layout-api";

describe("fetchHomeLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("reads the tenant route wrapper payload", async () => {
    vi.mocked(sessionFetch).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            overrides: { "section.birthdays": false },
            policies: { "tile.students_present": "required" },
            can_manage_policies: true,
          },
        }),
    } as Response);

    await expect(fetchHomeLayout()).resolves.toEqual({
      overrides: { "section.birthdays": false },
      policies: { "tile.students_present": "required" },
      canManagePolicies: true,
    });
  });

  it("uses the recommended layout only when access is unavailable", async () => {
    vi.mocked(sessionFetch).mockResolvedValue({
      ok: false,
      status: 403,
    } as Response);

    await expect(fetchHomeLayout()).resolves.toEqual({
      overrides: {},
      policies: {},
      canManagePolicies: false,
    });
  });

  it("throws network failures so SWR can retry the read", async () => {
    vi.mocked(sessionFetch).mockRejectedValue(new Error("network unavailable"));

    await expect(fetchHomeLayout()).rejects.toThrow("network unavailable");
  });

  it("throws unexpected HTTP failures so SWR can retry the read", async () => {
    vi.mocked(sessionFetch).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response);

    await expect(fetchHomeLayout()).rejects.toThrow(
      "home layout request failed (500)",
    );
  });
});
