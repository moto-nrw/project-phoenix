import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "../server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import type { ApiErrorResponse, ApiResponse } from "./api-helpers.server";
import { handleApiError } from "./api-helpers.server";

interface FileUploadOptions {
  maxSizeInMB?: number;
  allowedMimeTypes?: string[];
  allowedExtensions?: string[];
}

const MULTIPART_OVERHEAD_BYTES = 2 * 1024 * 1024;

/** Error thrown by file validation with the appropriate HTTP status code. */
export class FileValidationError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "FileValidationError";
    this.status = status;
  }
}

/**
 * Validates uploaded file against security constraints.
 * Throws FileValidationError with a semantically correct HTTP status:
 *   413 — file too large
 *   415 — wrong MIME type or extension
 *   422 — suspicious filename (double extensions, path traversal)
 */
function validateFile(file: File, options: FileUploadOptions): void {
  const {
    maxSizeInMB = 5,
    allowedMimeTypes = [
      "image/jpeg",
      "image/jpg",
      "image/png",
      "image/gif",
      "image/webp",
    ],
    allowedExtensions = [".jpg", ".jpeg", ".png", ".gif", ".webp"],
  } = options;

  // Check file size
  const maxSizeInBytes = maxSizeInMB * 1024 * 1024;
  if (file.size > maxSizeInBytes) {
    throw new FileValidationError(
      `File size exceeds ${maxSizeInMB}MB limit`,
      413,
    );
  }

  // Check MIME type
  if (!allowedMimeTypes.includes(file.type)) {
    throw new FileValidationError(
      `File type ${file.type} is not allowed. Allowed types: ${allowedMimeTypes.join(", ")}`,
      415,
    );
  }

  // Check file extension
  const fileName = file.name.toLowerCase();
  const hasValidExtension = allowedExtensions.some((ext) =>
    fileName.endsWith(ext),
  );
  if (!hasValidExtension) {
    throw new FileValidationError(
      `File extension not allowed. Allowed extensions: ${allowedExtensions.join(", ")}`,
      415,
    );
  }

  // Additional security checks
  // Check for double extensions
  const extensionCount = (fileName.match(/\./g) ?? []).length;
  if (extensionCount > 1) {
    throw new FileValidationError(
      "Files with multiple extensions are not allowed",
      422,
    );
  }

  // Check for suspicious patterns in filename
  if (
    fileName.includes("..") ||
    fileName.includes("/") ||
    fileName.includes("\\")
  ) {
    throw new FileValidationError("Invalid filename", 422);
  }
}

async function parseBoundedMultipartFormData(
  request: NextRequest,
  maxSizeInBytes: number,
): Promise<FormData> {
  const maxBodySize = maxSizeInBytes + MULTIPART_OVERHEAD_BYTES;
  const contentLength = request.headers.get("content-length");
  if (contentLength !== null && Number(contentLength) > maxBodySize) {
    throw new FileValidationError(
      `Request body exceeds ${maxSizeInBytes / (1024 * 1024)}MB limit`,
      413,
    );
  }

  if (!request.body) {
    return request.formData();
  }

  const reader = request.body.getReader();
  const chunks: ArrayBuffer[] = [];
  let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      size += value.byteLength;
      if (size > maxBodySize) {
        await reader.cancel();
        throw new FileValidationError(
          `Request body exceeds ${maxSizeInBytes / (1024 * 1024)}MB limit`,
          413,
        );
      }
      const chunk = new Uint8Array(value.byteLength);
      chunk.set(value);
      chunks.push(chunk.buffer);
    }
  } finally {
    reader.releaseLock();
  }

  return new Request(request.url, {
    method: request.method,
    headers: {
      "content-type": request.headers.get("content-type") ?? "",
    },
    body: new Blob(chunks),
  }).formData();
}

/**
 * Wrapper function for handling file upload API routes
 */
export function createFileUploadHandler<T>(
  handler: (
    request: NextRequest,
    formData: FormData,
    token: string,
    params: Record<string, unknown>,
  ) => Promise<T>,
  options?: FileUploadOptions,
) {
  return withTenantAuth(
    async (
      request,
      context: {
        params: Promise<Record<string, string | string[] | undefined>>;
      },
    ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
      try {
        const session = await auth();

        if (!session?.user?.token) {
          return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
        }

        // Extract parameters
        const safeParams: Record<string, unknown> = {};

        const contextParams = await context.params;
        if (contextParams) {
          Object.entries(contextParams).forEach(([key, value]) => {
            if (value !== undefined) {
              safeParams[key] = value;
            }
          });
        }

        // Get form data
        const maxSizeInBytes = (options?.maxSizeInMB ?? 5) * 1024 * 1024;
        const formData = await parseBoundedMultipartFormData(
          request,
          maxSizeInBytes,
        );

        // Validate all files in the form data — return the semantically correct
        // HTTP status (413/415/422) instead of letting validation errors fall
        // through to the generic 500 handler.
        try {
          for (const [, value] of formData.entries()) {
            if (value instanceof File) {
              validateFile(value, options ?? {});
            }
          }
        } catch (validationError) {
          if (validationError instanceof FileValidationError) {
            return NextResponse.json(
              { error: validationError.message },
              { status: validationError.status },
            );
          }
          throw validationError;
        }

        const data = await handler(
          request,
          formData,
          session.user.token,
          safeParams,
        );

        // Wrap the response in ApiResponse format if it's not already
        const response: ApiResponse<T> =
          typeof data === "object" && data !== null && "success" in data
            ? (data as unknown as ApiResponse<T>)
            : { success: true, message: "Success", data };

        return NextResponse.json(response);
      } catch (error) {
        if (error instanceof FileValidationError) {
          return NextResponse.json(
            { error: error.message },
            { status: error.status },
          );
        }
        return handleApiError(error);
      }
    },
  );
}
