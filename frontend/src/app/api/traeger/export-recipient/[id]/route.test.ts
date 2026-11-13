import { beforeEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

// One recipient of the finance exports (ADR 0218, ADR 0221) on the Träger
// session: read and save pass through, removing answers the page without
// it, so the list can show it at once. Not a pure proxy route, so the proxy
// table does not cover it.

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

import { DELETE, GET, PUT } from "./route";

beforeEach(() => {
  vi.clearAllMocks();
  mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
});

const params = { params: Promise.resolve({ id: "7" }) };

it("answers the page without the removed recipient", async () => {
  const emptied = { recipients: [], assignments: [], handovers: [] };
  mockFetch.mockResolvedValueOnce(
    Response.json({ status: "success", data: emptied, message: "ok" }),
  );

  const response = await DELETE(
    new NextRequest("http://localhost:3000/api/traeger/export-recipient/7", {
      method: "DELETE",
    }),
    params,
  );

  expect(response.status).toBe(200);
  expect(await response.json()).toMatchObject({ data: emptied });
  expect(mockFetch).toHaveBeenCalledWith(
    "http://localhost:8080/traeger/export-recipient/7",
    expect.objectContaining({
      method: "DELETE",
      headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
    }),
  );
});

it("reads and saves the recipient on the Träger session", async () => {
  const recipient = { id: 7, ready: true, password_set: true };
  mockFetch.mockImplementation(async () =>
    Response.json({ status: "success", data: recipient, message: "ok" }),
  );
  const url = "http://localhost:3000/api/traeger/export-recipient/7";

  const read = await GET(new NextRequest(url), params);
  const saved = await PUT(
    new NextRequest(url, {
      method: "PUT",
      body: JSON.stringify({ host: "h" }),
    }),
    params,
  );

  expect(read.status).toBe(200);
  expect(saved.status).toBe(200);
  expect(mockFetch).toHaveBeenNthCalledWith(
    1,
    "http://localhost:8080/traeger/export-recipient/7",
    expect.objectContaining({
      method: "GET",
      headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
    }),
  );
  expect(mockFetch).toHaveBeenNthCalledWith(
    2,
    "http://localhost:8080/traeger/export-recipient/7",
    expect.objectContaining({
      method: "PUT",
      body: JSON.stringify({ host: "h" }),
    }),
  );
});
