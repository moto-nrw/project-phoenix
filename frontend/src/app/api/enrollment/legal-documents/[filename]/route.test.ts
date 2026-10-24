import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { DELETE } from "./route";
import { ApiResponseError } from "~/lib/api-response-error.server";
import type { ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("../../../../../server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  const actual = await vi.importActual<
    typeof import("~/lib/api-helpers.server")
  >("~/lib/api-helpers.server");
  return {
    apiDelete: mockApiDelete,
    handleApiError: vi.fn(mockHandleApiError),
    problemResponse: actual.problemResponse,
  };
});

const session = (token: string): ExtendedSession => ({
  user: { id: "1", token, name: "Test User" },
  expires: "2099-01-01",
});

function request(path = "/api/enrollment/legal-documents/1_terms.pdf") {
  return new NextRequest(new URL(path, "http://localhost:3000"), {
    method: "DELETE",
  });
}

function context(filename?: string) {
  return {
    params: Promise.resolve(filename === undefined ? {} : { filename }),
  } as { params: Promise<Record<string, string | string[] | undefined>> };
}

describe("DELETE /api/enrollment/legal-documents/[filename]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(session("old-token"));
    mockUncachedAuth.mockResolvedValue(session("new-token"));
  });

  it("returns 401 when no session token is available", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await DELETE(request(), context("1_terms.pdf"));

    expect(response.status).toBe(401);
    expect(mockApiDelete).not.toHaveBeenCalled();
  });

  it("deletes the encoded filename via the backend", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const response = await DELETE(request(), context("1 terms.pdf"));

    expect(response.status).toBe(204);
    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/enrollment/legal-documents/1%20terms.pdf",
      "old-token",
    );
  });

  it("retries with uncachedAuth when the backend token is stale", async () => {
    mockApiDelete
      .mockRejectedValueOnce(new ApiResponseError(401, "Unauthorized"))
      .mockResolvedValueOnce(undefined);

    const response = await DELETE(request(), context("1_terms.pdf"));

    expect(response.status).toBe(204);
    expect(mockUncachedAuth).toHaveBeenCalledTimes(1);
    expect(mockApiDelete).toHaveBeenNthCalledWith(
      1,
      "/api/enrollment/legal-documents/1_terms.pdf",
      "old-token",
    );
    expect(mockApiDelete).toHaveBeenNthCalledWith(
      2,
      "/api/enrollment/legal-documents/1_terms.pdf",
      "new-token",
    );
  });

  it("answers token expired when the refreshed token is unchanged", async () => {
    mockUncachedAuth.mockResolvedValueOnce(session("old-token"));
    mockApiDelete.mockRejectedValueOnce(
      new ApiResponseError(401, "Unauthorized"),
    );

    const response = await DELETE(request(), context("1_terms.pdf"));
    const body = (await response.json()) as { code?: string; detail?: string };

    expect(response.status).toBe(401);
    expect(body.code).toBe("general.permission");
    expect(body.detail).toBe("Token expired");
    expect(mockApiDelete).toHaveBeenCalledTimes(1);
  });
});
