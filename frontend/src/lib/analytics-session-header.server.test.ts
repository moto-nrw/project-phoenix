import { describe, expect, it, vi } from "vitest";

const { mockNextHeaders } = vi.hoisted(() => ({
  mockNextHeaders: vi.fn(),
}));

vi.mock("next/headers", () => ({
  headers: mockNextHeaders,
}));

import {
  analyticsSessionHeaders,
  incomingAnalyticsSessionHeaders,
} from "./analytics-session-header.server";

const SESSION_ID = "0199a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b";

describe("analyticsSessionHeaders", () => {
  it("passes on the browser's session UUID", () => {
    expect(
      analyticsSessionHeaders(
        new Headers({ "x-posthog-session-id": ` ${SESSION_ID} ` }),
      ),
    ).toEqual({ "X-POSTHOG-SESSION-ID": SESSION_ID });
  });

  it("drops anything that is not a session UUID", () => {
    for (const value of ["", "Mia Müller", `${SESSION_ID}x`, "../etc"]) {
      expect(
        analyticsSessionHeaders(new Headers({ "x-posthog-session-id": value })),
      ).toEqual({});
    }
    expect(analyticsSessionHeaders(new Headers())).toEqual({});
    expect(analyticsSessionHeaders(null)).toEqual({});
  });
});

describe("incomingAnalyticsSessionHeaders", () => {
  it("reads the session of the request being handled", async () => {
    mockNextHeaders.mockResolvedValueOnce(
      new Headers({ "x-posthog-session-id": SESSION_ID }),
    );

    await expect(incomingAnalyticsSessionHeaders()).resolves.toEqual({
      "X-POSTHOG-SESSION-ID": SESSION_ID,
    });
  });

  it("has nothing to forward outside a request", async () => {
    mockNextHeaders.mockRejectedValueOnce(new Error("outside request scope"));

    await expect(incomingAnalyticsSessionHeaders()).resolves.toEqual({});
  });
});
