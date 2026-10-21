import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";
import {
  type ExtendedSession,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

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

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(formData: FormData): NextRequest {
  const url = new URL("/api/import/students/import", "http://localhost:3000");
  return new NextRequest(url, {
    method: "POST",
    body: formData,
  });
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/import/students/import", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("imports students via backend", async () => {
    const mockResult = {
      imported: 3,
      failed: 0,
      students: [
        { id: 1, first_name: "Max", second_name: "Mustermann" },
        { id: 2, first_name: "Anna", second_name: "Schmidt" },
        { id: 3, first_name: "Tom", second_name: "Weber" },
      ],
    };

    mockFetch.mockResolvedValueOnce(Response.json(mockResult));

    const formData = new FormData();
    const file = new Blob(
      [
        "first_name,second_name,class_name\nMax,Mustermann,1a\nAnna,Schmidt,1b\nTom,Weber,2a",
      ],
      { type: "text/csv" },
    );
    formData.append("file", file, "students.csv");

    const request = createMockRequest(formData);
    const response = await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/import/students/import",
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
