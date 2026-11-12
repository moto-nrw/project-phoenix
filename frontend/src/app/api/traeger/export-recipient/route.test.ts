import { beforeEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

// The recipient of the finance exports (ADR 0218) on the Träger session:
// read and save pass through, removing answers the emptied recipient, so the
// page can show it at once. Not a pure proxy route, so the proxy table does
// not cover it.

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

it("reads and saves the recipient on the Träger session", async () => {
  const recipient = { configured: true, ready: true, password_set: true };
  mockFetch.mockImplementation(async () =>
    Response.json({ status: "success", data: recipient, message: "ok" }),
  );
  const url = "http://localhost:3000/api/traeger/export-recipient";

  const read = await GET(new NextRequest(url), { params: Promise.resolve({}) });
  const saved = await PUT(
    new NextRequest(url, {
      method: "PUT",
      body: JSON.stringify({ host: "h" }),
    }),
    { params: Promise.resolve({}) },
  );

  expect(read.status).toBe(200);
  expect(saved.status).toBe(200);
  expect(mockFetch).toHaveBeenNthCalledWith(
    1,
    "http://localhost:8080/traeger/export-recipient/",
    expect.objectContaining({
      method: "GET",
      headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
    }),
  );
  expect(mockFetch).toHaveBeenNthCalledWith(
    2,
    "http://localhost:8080/traeger/export-recipient/",
    expect.objectContaining({
      method: "PUT",
      body: JSON.stringify({ host: "h" }),
    }),
  );
});
