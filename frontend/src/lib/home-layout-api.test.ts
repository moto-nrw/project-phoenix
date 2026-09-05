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
});
