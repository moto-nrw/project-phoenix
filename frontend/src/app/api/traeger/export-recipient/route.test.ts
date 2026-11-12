import { beforeEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

// Removing the recipient (ADR 0218) answers the emptied recipient, so the
// page can show it at once.

const { mockTraegerAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockTraegerAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/traeger", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("traeger", {
    auth: mockTraegerAuth,
    uncachedAuth: mockUncachedAuth,
  }),
);
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));
global.fetch = mockFetch as typeof fetch;

import { DELETE } from "./route";

beforeEach(() => {
  vi.clearAllMocks();
  mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
});

it("answers the emptied recipient of the backend", async () => {
  const emptied = { configured: false, ready: false, password_set: false };
  mockFetch.mockResolvedValueOnce(
    Response.json({ status: "success", data: emptied, message: "ok" }),
  );

  const response = await DELETE(
    new NextRequest("http://localhost:3000/api/traeger/export-recipient", {
      method: "DELETE",
    }),
    { params: Promise.resolve({}) },
  );

  expect(response.status).toBe(200);
  expect(await response.json()).toMatchObject({ data: emptied });
  expect(mockFetch).toHaveBeenCalledWith(
    "http://localhost:8080/traeger/export-recipient/",
    expect.objectContaining({
      method: "DELETE",
      headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
    }),
  );
});
