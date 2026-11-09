import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { problemResponse } from "./api-response-error.server";
import {
  readBackendJson,
  sendBackend,
  unwrapEnvelope,
} from "./backend-client.server";
import type { ErrorCode } from "./error-codes.generated";
import type { ProxyEndpoint } from "./portal-handlers.server";
import {
  readBoundedBody,
  wrapInApiResponse,
} from "./route-wrapper-utils.server";
import { tenant } from "./route-wrapper.server";

interface FileUploadOptions {
  maxSizeInMB?: number;
  allowedMimeTypes?: string[];
  allowedExtensions?: string[];
}

const MULTIPART_OVERHEAD_BYTES = 2 * 1024 * 1024;

/**
 * A scanned or photographed paper document. An empty or generic MIME type is
 * what some browsers send for a valid PDF; the extension gate and the
 * backend's content check still apply.
 */
export const SCAN_UPLOAD: FileUploadOptions = {
  maxSizeInMB: 10,
  allowedMimeTypes: [
    "",
    "application/octet-stream",
    "application/pdf",
    "image/png",
    "image/jpeg",
    "image/jpg",
  ],
  allowedExtensions: [".pdf", ".png", ".jpg", ".jpeg"],
};

/**
 * A scan or a Word document. DOCX is an OOXML ZIP container, so browsers
 * without the OOXML mapping label a valid .docx as a plain ZIP; the backend
 * rejects an archive that lacks the OOXML parts (api/common/upload_documents.go).
 */
export const DOCUMENT_UPLOAD: FileUploadOptions = {
  maxSizeInMB: 10,
  allowedMimeTypes: [
    ...(SCAN_UPLOAD.allowedMimeTypes ?? []),
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
    "application/zip",
    "application/x-zip-compressed",
  ],
  allowedExtensions: [...(SCAN_UPLOAD.allowedExtensions ?? []), ".docx"],
};

/** Error thrown by file validation with the appropriate HTTP status code. */
export class FileValidationError extends Error {
  readonly status: number;
  readonly code: ErrorCode;
  readonly details?: Record<string, unknown>;

  constructor(
    message: string,
    status: number,
    code: ErrorCode,
    details?: Record<string, unknown>,
  ) {
    super(message);
    this.name = "FileValidationError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

/**
 * Validates uploaded file against security constraints.
 * Throws FileValidationError with a semantically correct HTTP status:
 *   413 — file too large
 *   415 — wrong MIME type or extension
 *   422 — suspicious filename (path traversal)
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
      "files.file_too_large",
      { max_mb: maxSizeInMB },
    );
  }

  // Check MIME type
  if (!allowedMimeTypes.includes(file.type)) {
    throw new FileValidationError(
      `File type ${file.type} is not allowed. Allowed types: ${allowedMimeTypes.join(", ")}`,
      415,
      "files.file_type_not_allowed",
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
      "files.file_type_not_allowed",
    );
  }

  // Reject path traversal. Multiple dots are valid in ordinary display names;
  // the backend verifies content and replaces the stored name with a UUID.
  if (
    fileName.includes("..") ||
    fileName.includes("/") ||
    fileName.includes("\\")
  ) {
    throw new FileValidationError(
      "Invalid filename",
      422,
      "files.file_name_invalid",
    );
  }
}

/** Reads a multipart body, rejecting it once it outgrows the file limit. */
export async function parseBoundedMultipartFormData(
  request: NextRequest,
  maxSizeInBytes: number,
): Promise<FormData> {
  const bytes = await readBoundedBody(
    request,
    maxSizeInBytes + MULTIPART_OVERHEAD_BYTES,
  );
  if (bytes === null) {
    throw new FileValidationError(
      `Request body exceeds ${maxSizeInBytes / (1024 * 1024)}MB limit`,
      413,
      "files.file_too_large",
      { max_mb: maxSizeInBytes / (1024 * 1024) },
    );
  }
  return new Request(request.url, {
    method: request.method,
    headers: { "content-type": request.headers.get("content-type") ?? "" },
    body: bytes,
  }).formData();
}

/** Sends a multipart form to the tenant backend and answers its `data`. */
export async function postForm<T>(
  request: NextRequest,
  path: string,
  token: string,
  form: FormData,
): Promise<T> {
  const response = await sendBackend(path, token, {
    method: "POST",
    body: form,
    request,
  });
  return unwrapEnvelope<T>(await readBackendJson(response));
}

/**
 * Tenant file upload route: authenticates, bounds and validates the
 * multipart body, then runs the handler, once more with the refreshed
 * session's token after it throws a backend 401. The handler builds its
 * outgoing form on each run, so the retry sends the same files again.
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
  return tenant.route(async (request, token, params) => {
    try {
      const maxSizeInBytes = (options?.maxSizeInMB ?? 5) * 1024 * 1024;
      const formData = await parseBoundedMultipartFormData(
        request,
        maxSizeInBytes,
      );
      for (const [, value] of formData.entries()) {
        if (value instanceof File) validateFile(value, options ?? {});
      }
      const result = await tenant.runWithRenewal(request, token, (current) =>
        handler(request, formData, current, params),
      );
      return result instanceof Response
        ? result
        : NextResponse.json(wrapInApiResponse(result));
    } catch (error) {
      if (error instanceof FileValidationError) {
        return fileValidationProblemResponse(error, request);
      }
      throw error;
    }
  });
}

interface FileForwardOptions extends FileUploadOptions {
  /** The form field carrying the file. */
  readonly file: string;
  /** Text fields the backend requires; a missing one answers 400. */
  readonly fields?: readonly string[];
  /** Text fields forwarded only when present. */
  readonly optionalFields?: readonly string[];
  /** Content check on the file bytes; throws FileValidationError. */
  readonly verify?: (bytes: ArrayBuffer) => void;
}

/**
 * Tenant upload proxy: validates the file and forwards it with the listed
 * fields and the client identity. Answers the backend's `data`.
 */
export function proxyFileUpload(
  endpoint: ProxyEndpoint,
  options: FileForwardOptions,
) {
  return createFileUploadHandler<unknown>(
    async (request, formData, token, params) => {
      const file = formData.get(options.file);
      if (!(file instanceof File)) {
        throw new FileValidationError(
          `No ${options.file} file provided`,
          400,
          "files.file_missing",
        );
      }
      options.verify?.(await file.arrayBuffer());

      const forwarded = new FormData();
      forwarded.append(options.file, file);
      for (const name of options.fields ?? []) {
        const value = formData.get(name);
        if (typeof value !== "string" || value === "") {
          throw new FileValidationError(
            `No ${name} provided`,
            400,
            "general.input",
          );
        }
        forwarded.append(name, value);
      }
      for (const name of options.optionalFields ?? []) {
        const value = formData.get(name);
        if (typeof value === "string" && value !== "") {
          forwarded.append(name, value);
        }
      }

      const path = typeof endpoint === "string" ? endpoint : endpoint(params);
      return postForm(request, path, token, forwarded);
    },
    options,
  );
}

export function fileValidationProblemResponse(
  error: FileValidationError,
  request: Request,
) {
  return problemResponse(
    error.status,
    error.message,
    {
      code: error.code,
      ...(error.details ? { details: error.details } : {}),
    },
    request,
  );
}
