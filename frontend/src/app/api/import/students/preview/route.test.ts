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
  const url = new URL("/api/import/students/preview", "http://localhost:3000");
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

describe("POST /api/import/students/preview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("forwards FormData to backend for preview", async () => {
    const mockPreview = {
      valid_students: [
        { first_name: "Max", second_name: "Mustermann", class_name: "1a" },
      ],
      invalid_rows: [],
    };

    mockFetch.mockResolvedValueOnce(Response.json(mockPreview));

    const formData = new FormData();
    const file = new Blob(
      ["first_name,second_name,class_name\nMax,Mustermann,1a"],
      {
        type: "text/csv",
      },
    );
    formData.append("file", file, "students.csv");

    const request = createMockRequest(formData);
    const response = await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/import/students/preview",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse(response);
    expect(json).toEqual(mockPreview);
  });
});
