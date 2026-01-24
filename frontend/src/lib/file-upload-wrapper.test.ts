import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";
import { createFileUploadHandler } from "./file-upload-wrapper";

// ============================================================================
// Constants
// ============================================================================

const TEST_COOKIE_HEADER = "better-auth.session_token=test-session-token";

// ============================================================================
// Mocks
// ============================================================================

// handleApiError mock
vi.mock("./api-helpers", () => ({
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

// ============================================================================
// Test Helpers
// ============================================================================

/**
 * Creates a mock File object for testing
 */
function createMockFile(name: string, size: number, type: string): File {
  const content = new ArrayBuffer(size);
  return new File([content], name, { type });
}

/**
 * Creates a mock NextRequest with FormData for file uploads
 */
function createMockUploadRequest(
  path: string,
  files: File[] = [],
  additionalData: Record<string, string> = {},
): NextRequest {
  const formData = new FormData();

  files.forEach((file, index) => {
    formData.append(`file${index}`, file);
  });

  Object.entries(additionalData).forEach(([key, value]) => {
    formData.append(key, value);
  });

  const url = new URL(path, "http://localhost:3000");

  // Create a request with the form data
  return new NextRequest(url, {
    method: "POST",
    body: formData,
  });
}

/**
 * Creates a mock route context with params (Next.js 15+ pattern)
 */
function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

/**
 * Helper to parse JSON response with proper typing
 */
async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

/**
 * Types for API responses
 */
interface ApiSuccessResponse<T = unknown> {
  success: boolean;
  message: string;
  data: T;
}

interface ApiErrorResponse {
  error: string;
}

// ============================================================================
// Tests: createFileUploadHandler - Authentication
// ============================================================================

describe("createFileUploadHandler - Authentication", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns 401 when user is not authenticated", async () => {
    // Mock auth to return null user
    const { auth } = await import("~/server/auth");
    vi.mocked(auth).mockResolvedValueOnce(null);

    const handler = createFileUploadHandler(async () => ({ uploaded: true }));
    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toBe("Unauthorized");
  });

  it("returns 401 when session has no user", async () => {
    // Mock auth to return session without user
    const { auth } = await import("~/server/auth");
    vi.mocked(auth).mockResolvedValueOnce({ user: null } as never);

    const handler = createFileUploadHandler(async () => ({ uploaded: true }));
    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toBe("Unauthorized");
  });
});

// ============================================================================
// Tests: createFileUploadHandler - File Validation
// ============================================================================

describe("createFileUploadHandler - File Validation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("rejects files exceeding size limit", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }), {
      maxSizeInMB: 1,
    });

    // Create file larger than 1MB
    const file = createMockFile("large.jpg", 2 * 1024 * 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("File size exceeds 1MB limit");
  });

  it("uses default 5MB size limit when not specified", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }));

    // Create file larger than 5MB
    const file = createMockFile("large.jpg", 6 * 1024 * 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("File size exceeds 5MB limit");
  });

  it("accepts files within size limit", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler, { maxSizeInMB: 5 });

    // Create file smaller than 5MB
    const file = createMockFile("small.jpg", 1024 * 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });

  it("rejects files with invalid MIME type", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }));

    const file = createMockFile("test.exe", 1024, "application/x-msdownload");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain(
      "File type application/x-msdownload is not allowed",
    );
  });

  it("accepts files with valid MIME types", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler);

    const validTypes = [
      { name: "test.jpg", type: "image/jpeg" },
      { name: "test.png", type: "image/png" },
      { name: "test.gif", type: "image/gif" },
      { name: "test.webp", type: "image/webp" },
    ];

    for (const { name, type } of validTypes) {
      vi.clearAllMocks();
      const file = createMockFile(name, 1024, type);
      const request = createMockUploadRequest("/api/upload", [file]);
      const response = await handler(request, createMockContext());

      expect(response.status).toBe(200);
    }
  });

  it("allows custom MIME types when specified", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler, {
      allowedMimeTypes: ["application/pdf"],
      allowedExtensions: [".pdf"],
    });

    const file = createMockFile("document.pdf", 1024, "application/pdf");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });

  it("rejects files with invalid extension", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }));

    // File with valid MIME but invalid extension
    const file = createMockFile("test.txt", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("File extension not allowed");
  });

  it("rejects files with double extensions", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }));

    // Use valid extension for both parts so it fails on double extension check
    const file = createMockFile("test.jpg.png", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain(
      "Files with multiple extensions are not allowed",
    );
  });

  it("rejects filenames with double dot patterns", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }));

    // Files with ".." are rejected either due to double extension or invalid filename
    // The validation catches these for security reasons
    const maliciousNames = ["../test.jpg", "..\\test.jpg"];

    for (const name of maliciousNames) {
      const file = createMockFile(name, 1024, "image/jpeg");
      const request = createMockUploadRequest("/api/upload", [file]);
      const response = await handler(request, createMockContext());

      expect(response.status).toBe(500);
      // Either "multiple extensions" or "Invalid filename" - both block the attack
      const json = await parseJsonResponse<ApiErrorResponse>(response);
      expect(json.error).toBeTruthy();
    }
  });

  it("rejects filenames with forward slashes", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }));

    const file = createMockFile("path/to/test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid filename");
  });

  it("rejects filenames with backslashes", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }));

    const file = createMockFile("path\\to\\test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid filename");
  });
});

// ============================================================================
// Tests: createFileUploadHandler - Handler Execution
// ============================================================================

describe("createFileUploadHandler - Handler Execution", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls handler with request, formData, cookieHeader, and params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true, id: 123 });

    const handler = createFileUploadHandler(mockHandler);
    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload/456", [file], {
      description: "Test upload",
    });
    await handler(request, createMockContext({ id: "456" }));

    expect(mockHandler).toHaveBeenCalledWith(
      request,
      expect.any(FormData),
      TEST_COOKIE_HEADER,
      expect.objectContaining({ id: "456" }),
    );
  });

  it("extracts params from context correctly", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });

    const handler = createFileUploadHandler(mockHandler);
    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/groups/123/upload", [file]);
    await handler(
      request,
      createMockContext({ groupId: "123", type: "avatar" }),
    );

    const calledParams = mockHandler.mock.calls[0]?.[3] ?? {};
    expect(calledParams).toEqual({ groupId: "123", type: "avatar" });
  });

  it("filters out undefined params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });

    const handler = createFileUploadHandler(mockHandler);
    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    await handler(
      request,
      createMockContext({ id: "123", optional: undefined }),
    );

    const calledParams = mockHandler.mock.calls[0]?.[3] ?? {};
    expect(calledParams).toEqual({ id: "123" });
    expect(Object.prototype.hasOwnProperty.call(calledParams, "optional")).toBe(
      false,
    );
  });

  it("validates all files in form data", async () => {
    const handler = createFileUploadHandler(async () => ({ uploaded: true }), {
      maxSizeInMB: 1,
    });

    // First file is valid, second exceeds size limit
    const validFile = createMockFile("small.jpg", 512 * 1024, "image/jpeg");
    const invalidFile = createMockFile(
      "large.jpg",
      2 * 1024 * 1024,
      "image/jpeg",
    );

    const formData = new FormData();
    formData.append("file1", validFile);
    formData.append("file2", invalidFile);

    const url = new URL("/api/upload", "http://localhost:3000");
    const request = new NextRequest(url, {
      method: "POST",
      body: formData,
    });

    const response = await handler(request, createMockContext());
    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("File size exceeds 1MB limit");
  });

  it("ignores non-file form data entries during validation", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler);

    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file], {
      title: "My Image",
      description: "A test image",
    });

    const response = await handler(request, createMockContext());
    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });
});

// ============================================================================
// Tests: createFileUploadHandler - Response Wrapping
// ============================================================================

describe("createFileUploadHandler - Response Wrapping", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("wraps response in ApiResponse format", async () => {
    const handler = createFileUploadHandler(async () => ({
      id: 1,
      filename: "test.jpg",
      size: 1024,
    }));

    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiSuccessResponse>(response);
    expect(json).toEqual({
      success: true,
      message: "Success",
      data: { id: 1, filename: "test.jpg", size: 1024 },
    });
  });

  it("passes through response if already wrapped with success field", async () => {
    const alreadyWrapped = {
      success: true,
      message: "Custom message",
      data: { uploaded: true },
    };
    const handler = createFileUploadHandler(async () => alreadyWrapped);

    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    const json = await parseJsonResponse<ApiSuccessResponse>(response);
    expect(json).toEqual(alreadyWrapped);
  });

  it("handles null response from handler", async () => {
    const handler = createFileUploadHandler(async () => null);

    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiSuccessResponse>(response);
    expect(json).toEqual({
      success: true,
      message: "Success",
      data: null,
    });
  });
});

// ============================================================================
// Tests: createFileUploadHandler - Error Handling
// ============================================================================

describe("createFileUploadHandler - Error Handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("handles handler errors through handleApiError", async () => {
    const handler = createFileUploadHandler(async () => {
      throw new Error("Upload processing failed");
    });

    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Upload processing failed");
  });

  it("handles API errors with status codes", async () => {
    const handler = createFileUploadHandler(async () => {
      throw new Error("API error (404): File not found");
    });

    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(404);
  });
});

// ============================================================================
// Tests: createFileUploadHandler - Edge Cases
// ============================================================================

describe("createFileUploadHandler - Edge Cases", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("handles form data without any files", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ processed: true });
    const handler = createFileUploadHandler(mockHandler);

    const request = createMockUploadRequest("/api/upload", [], {
      metadata: "some data",
    });
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });

  it("handles empty context params", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler);

    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    await handler(request, createMockContext({}));

    const calledParams = mockHandler.mock.calls[0]?.[3] ?? {};
    expect(calledParams).toEqual({});
  });

  it("uses default options when none provided", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler);

    // Valid file with default options
    const file = createMockFile("test.jpg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });

  it("validates file extension case-insensitively", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler);

    // Uppercase extension should still work
    const file = createMockFile("TEST.JPG", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });

  it("accepts JPEG with .jpeg extension", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler);

    const file = createMockFile("test.jpeg", 1024, "image/jpeg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });

  it("accepts image/jpg MIME type", async () => {
    const mockHandler = vi.fn().mockResolvedValue({ uploaded: true });
    const handler = createFileUploadHandler(mockHandler);

    const file = createMockFile("test.jpg", 1024, "image/jpg");
    const request = createMockUploadRequest("/api/upload", [file]);
    const response = await handler(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockHandler).toHaveBeenCalled();
  });
});
