import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";
import {
  type ExtendedSession,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

const mockFetch = vi.fn();
global.fetch = mockFetch;

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

function createMockRequest(formData: FormData): NextRequest {
  const url = new URL("/api/import/teachers/import", "http://localhost:3000");
  return new NextRequest(url, { method: "POST", body: formData });
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("POST /api/import/teachers/import", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("imports staff via backend", async () => {
    const mockResult = {
      data: { TotalRows: 2, CreatedCount: 2, ErrorCount: 0, Errors: [] },
    };

    mockFetch.mockResolvedValueOnce(Response.json(mockResult));

    const formData = new FormData();
    formData.append(
      "file",
      new Blob(
        [
          "Vorname,Nachname,Email,Rolle\nAnna,L,a@b.de,user\nBoris,K,b@c.de,admin",
        ],
        { type: "text/csv" },
      ),
      "staff.csv",
    );

    const response = await POST(createMockRequest(formData));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/import/teachers/import",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse(response);
    expect(json).toEqual(mockResult);
  });
});
