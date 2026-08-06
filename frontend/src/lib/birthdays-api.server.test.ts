import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  fetchBirthdayOptOut,
  fetchBirthdayOverview,
  updateBirthdayOptOut,
} from "./birthdays-api.server";
import { apiGet, apiPut } from "./api-helpers.server";

vi.mock("./api-helpers.server", () => ({
  apiGet: vi.fn(),
  apiPut: vi.fn(),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("fetchBirthdayOverview", () => {
  it("unwraps the backend envelope and forwards the payload verbatim", async () => {
    const payload = {
      enabled: true,
      include_staff: false,
      today: "2026-08-05",
      celebrations: [],
    };
    vi.mocked(apiGet).mockResolvedValue({ data: payload });

    await expect(fetchBirthdayOverview("token")).resolves.toBe(payload);
    expect(apiGet).toHaveBeenCalledWith("/api/birthdays", "token");
  });

  it("propagates a backend failure so the route can map the status", async () => {
    vi.mocked(apiGet).mockRejectedValue(new Error("401 Unauthorized"));

    await expect(fetchBirthdayOverview("token")).rejects.toThrow(
      "401 Unauthorized",
    );
  });
});

describe("fetchBirthdayOptOut", () => {
  it("reads the caller's own preference", async () => {
    vi.mocked(apiGet).mockResolvedValue({ data: { opt_out: true } });

    await expect(fetchBirthdayOptOut("token")).resolves.toEqual({
      opt_out: true,
    });
    expect(apiGet).toHaveBeenCalledWith("/api/birthdays/opt-out", "token");
  });
});

describe("updateBirthdayOptOut", () => {
  // The body carries the preference only — the person comes from the token,
  // so no caller can write someone else's setting.
  it("sends only the preference, in the argument order apiPut expects", async () => {
    vi.mocked(apiPut).mockResolvedValue({ data: { opt_out: true } });

    await expect(updateBirthdayOptOut("token", true)).resolves.toEqual({
      opt_out: true,
    });
    expect(apiPut).toHaveBeenCalledWith("/api/birthdays/opt-out", "token", {
      opt_out: true,
    });
  });

  it("passes a disabled preference through unchanged", async () => {
    vi.mocked(apiPut).mockResolvedValue({ data: { opt_out: false } });

    await updateBirthdayOptOut("token", false);

    expect(apiPut).toHaveBeenCalledWith("/api/birthdays/opt-out", "token", {
      opt_out: false,
    });
  });
});
