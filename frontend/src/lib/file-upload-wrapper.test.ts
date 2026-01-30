import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest, NextResponse } from "next/server";
import { createFileUploadHandler } from "./file-upload-wrapper";

interface ErrorResponse {
  error: string;
  status?: number;
}

interface SuccessResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

// Mock auth module
vi.mock("../server/auth/index", () => ({
  auth: vi.fn(),
}));

// Mock api-helpers module
vi.mock("./api-helpers", () => ({
  handleApiError: vi.fn((error) => {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : String(error) },
      { status: 500 },
    );
  }),
}));

import { auth } from "../server/auth/index";

describe("createFileUploadHandler", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("authentication", () => {
    it("returns 401 when session is missing", async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- mock function needs flexible typing
      (auth as any).mockResolvedValue(null);

      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      const request = new NextRequest("http://localhost:3000/api/upload");
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(401);
      expect(data).toEqual({ error: "Unauthorized" });
      expect(handler).not.toHaveBeenCalled();
    });

    it("returns 401 when session has no token", async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- mock function needs flexible typing
      (auth as any).mockResolvedValue({
        user: {},
      });

      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      const request = new NextRequest("http://localhost:3000/api/upload");
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(401);
      expect(data).toEqual({ error: "Unauthorized" });
      expect(handler).not.toHaveBeenCalled();
    });

    it("proceeds when session has valid token", async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- mock function needs flexible typing
      (auth as any).mockResolvedValue({
        user: { token: "valid-token" },
      });

      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      await wrappedHandler(request, context);

      expect(handler).toHaveBeenCalledWith(
        request,
        expect.any(FormData),
        "valid-token",
        {},
      );
    });
  });

  describe("file validation", () => {
    beforeEach(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- mock function needs flexible typing
      (auth as any).mockResolvedValue({
        user: { token: "valid-token" },
      });
    });

    it("rejects files exceeding size limit (default 5MB)", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      // Create 6MB file
      const largeFile = new File(
        [new ArrayBuffer(6 * 1024 * 1024)],
        "large.jpg",
        {
          type: "image/jpeg",
        },
      );

      const formData = new FormData();
      formData.append("file", largeFile);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toContain("File size exceeds 5MB limit");
      expect(handler).not.toHaveBeenCalled();
    });

    it("accepts files within size limit", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      // Create 1MB file
      const smallFile = new File(
        [new ArrayBuffer(1 * 1024 * 1024)],
        "small.jpg",
        {
          type: "image/jpeg",
        },
      );

      const formData = new FormData();
      formData.append("file", smallFile);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      expect(response.status).toBe(200);
      expect(handler).toHaveBeenCalled();
    });

    it("accepts custom size limit", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler, {
        maxSizeInMB: 10,
      });

      // Create 8MB file
      const file = new File([new ArrayBuffer(8 * 1024 * 1024)], "medium.jpg", {
        type: "image/jpeg",
      });

      const formData = new FormData();
      formData.append("file", file);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      expect(response.status).toBe(200);
      expect(handler).toHaveBeenCalled();
    });

    it("rejects disallowed MIME types", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      const textFile = new File(["content"], "file.txt", {
        type: "text/plain",
      });

      const formData = new FormData();
      formData.append("file", textFile);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toContain("File type text/plain is not allowed");
      expect(handler).not.toHaveBeenCalled();
    });

    it("accepts allowed MIME types (JPEG)", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const jpegFile = new File([new ArrayBuffer(100)], "photo.jpeg", {
        type: "image/jpeg",
      });

      const formData = new FormData();
      formData.append("file", jpegFile);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      expect(response.status).toBe(200);
    });

    it("accepts allowed MIME types (PNG)", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const pngFile = new File([new ArrayBuffer(100)], "image.png", {
        type: "image/png",
      });

      const formData = new FormData();
      formData.append("file", pngFile);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      expect(response.status).toBe(200);
    });

    it("accepts custom MIME types", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler, {
        allowedMimeTypes: ["application/pdf"],
        allowedExtensions: [".pdf"],
      });

      const pdfFile = new File([new ArrayBuffer(100)], "document.pdf", {
        type: "application/pdf",
      });

      const formData = new FormData();
      formData.append("file", pdfFile);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      expect(response.status).toBe(200);
    });

    it("rejects disallowed file extensions", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      const file = new File([new ArrayBuffer(100)], "file.pdf", {
        type: "image/jpeg", // MIME type matches but extension doesn't
      });

      const formData = new FormData();
      formData.append("file", file);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toContain("File extension not allowed");
      expect(handler).not.toHaveBeenCalled();
    });

    it("rejects files with double extensions", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      // Use valid extension so it passes extension check but fails double-extension check
      const file = new File([new ArrayBuffer(100)], "file.png.jpg", {
        type: "image/jpeg",
      });

      const formData = new FormData();
      formData.append("file", file);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toContain(
        "Files with multiple extensions are not allowed",
      );
      expect(handler).not.toHaveBeenCalled();
    });

    it("rejects files with path traversal in filename (..)", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      // Single dot in name so it passes double-extension check, but has path traversal
      const file = new File([new ArrayBuffer(100)], "..%2Fevil.jpg", {
        type: "image/jpeg",
      });

      const formData = new FormData();
      formData.append("file", file);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      // The file either fails extension check or traversal check
      expect(response.status).toBe(500);
      expect(handler).not.toHaveBeenCalled();
    });

    it("rejects files with forward slash in filename", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      const file = new File([new ArrayBuffer(100)], "path/to/file.jpg", {
        type: "image/jpeg",
      });

      const formData = new FormData();
      formData.append("file", file);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toBe("Invalid filename");
      expect(handler).not.toHaveBeenCalled();
    });

    it("rejects files with backslash in filename", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      const file = new File([new ArrayBuffer(100)], "path\\file.jpg", {
        type: "image/jpeg",
      });

      const formData = new FormData();
      formData.append("file", file);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toBe("Invalid filename");
      expect(handler).not.toHaveBeenCalled();
    });

    it("validates all files in form data", async () => {
      const handler = vi.fn();
      const wrappedHandler = createFileUploadHandler(handler);

      const validFile = new File([new ArrayBuffer(100)], "valid.jpg", {
        type: "image/jpeg",
      });
      const invalidFile = new File([new ArrayBuffer(100)], "invalid.txt", {
        type: "text/plain",
      });

      const formData = new FormData();
      formData.append("file1", validFile);
      formData.append("file2", invalidFile);

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toContain("File type text/plain is not allowed");
      expect(handler).not.toHaveBeenCalled();
    });

    it("skips validation for non-file form entries", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const file = new File([new ArrayBuffer(100)], "photo.jpg", {
        type: "image/jpeg",
      });

      const formData = new FormData();
      formData.append("file", file);
      formData.append("name", "John Doe");
      formData.append("description", "Profile photo");

      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      expect(response.status).toBe(200);
      expect(handler).toHaveBeenCalled();
    });
  });

  describe("parameter extraction", () => {
    beforeEach(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- mock function needs flexible typing
      (auth as any).mockResolvedValue({
        user: { token: "valid-token" },
      });
    });

    it("extracts string params", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = {
        params: Promise.resolve({ id: "123", name: "test" }),
      };

      await wrappedHandler(request, context);

      expect(handler).toHaveBeenCalledWith(
        request,
        expect.any(FormData),
        "valid-token",
        { id: "123", name: "test" },
      );
    });

    it("filters out undefined params", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = {
        params: Promise.resolve({ id: "123", missing: undefined }),
      };

      await wrappedHandler(request, context);

      expect(handler).toHaveBeenCalledWith(
        request,
        expect.any(FormData),
        "valid-token",
        { id: "123" },
      );
    });

    it("handles empty params object", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      await wrappedHandler(request, context);

      expect(handler).toHaveBeenCalledWith(
        request,
        expect.any(FormData),
        "valid-token",
        {},
      );
    });
  });

  describe("response formatting", () => {
    beforeEach(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- mock function needs flexible typing
      (auth as any).mockResolvedValue({
        user: { token: "valid-token" },
      });
    });

    it("wraps raw data in ApiResponse format", async () => {
      const handler = vi.fn().mockResolvedValue({ userId: 123 });
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as SuccessResponse<{
        userId: number;
      }>;

      expect(data).toEqual({
        success: true,
        message: "Success",
        data: { userId: 123 },
      });
    });

    it("preserves existing ApiResponse format", async () => {
      const handler = vi.fn().mockResolvedValue({
        success: true,
        message: "Custom message",
        data: { userId: 123 },
      });
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as SuccessResponse<{
        userId: number;
      }>;

      expect(data).toEqual({
        success: true,
        message: "Custom message",
        data: { userId: 123 },
      });
    });

    it("returns JSON response with 200 status", async () => {
      const handler = vi.fn().mockResolvedValue({ success: true });
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);

      expect(response.status).toBe(200);
      expect(response.headers.get("content-type")).toContain(
        "application/json",
      );
    });
  });

  describe("error handling", () => {
    beforeEach(() => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access -- mock function needs flexible typing
      (auth as any).mockResolvedValue({
        user: { token: "valid-token" },
      });
    });

    it("handles handler errors via handleApiError", async () => {
      const handler = vi.fn().mockRejectedValue(new Error("Upload failed"));
      const wrappedHandler = createFileUploadHandler(handler);

      const formData = new FormData();
      const request = new NextRequest("http://localhost:3000/api/upload", {
        method: "POST",
        body: formData,
      });
      const context = { params: Promise.resolve({}) };

      const response = await wrappedHandler(request, context);
      const data = (await response.json()) as ErrorResponse;

      expect(response.status).toBe(500);
      expect(data.error).toBe("Upload failed");
    });
  });
});
