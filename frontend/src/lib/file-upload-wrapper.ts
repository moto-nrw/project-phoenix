import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "../server/auth";
import type { ApiErrorResponse, ApiResponse } from "./api-helpers";
import { handleApiError } from "./api-helpers";

interface FileUploadOptions {
  maxSizeInMB?: number;
  allowedMimeTypes?: string[];
  allowedExtensions?: string[];
}

/**
 * Validates uploaded file against security constraints
 */
function validateFile(file: File, options: FileUploadOptions): void {
  const { 
    maxSizeInMB = 5, 
    allowedMimeTypes = ['image/jpeg', 'image/jpg', 'image/png', 'image/gif', 'image/webp'],
    allowedExtensions = ['.jpg', '.jpeg', '.png', '.gif', '.webp']
  } = options;

  // Check file size
  const maxSizeInBytes = maxSizeInMB * 1024 * 1024;
  if (file.size > maxSizeInBytes) {
    throw new Error(`File size exceeds ${maxSizeInMB}MB limit`);
  }

  // Check MIME type
  if (!allowedMimeTypes.includes(file.type)) {
    throw new Error(`File type ${file.type} is not allowed. Allowed types: ${allowedMimeTypes.join(', ')}`);
  }

  // Check file extension
  const fileName = file.name.toLowerCase();
  const hasValidExtension = allowedExtensions.some(ext => fileName.endsWith(ext));
  if (!hasValidExtension) {
    throw new Error(`File extension not allowed. Allowed extensions: ${allowedExtensions.join(', ')}`);
  }

  // Additional security checks
  // Check for double extensions
  const extensionCount = (fileName.match(/\./g) ?? []).length;
  if (extensionCount > 1) {
    throw new Error("Files with multiple extensions are not allowed");
  }

  // Check for suspicious patterns in filename
  if (fileName.includes('..') || fileName.includes('/') || fileName.includes('\\')) {
    throw new Error("Invalid filename");
  }
}

/**
 * Wrapper function for handling file upload API routes
 */
export function createFileUploadHandler<T>(
  handler: (
    request: NextRequest,
    formData: FormData,
    token: string,
    params: Record<string, unknown>
  ) => Promise<T>,
  options?: FileUploadOptions
) {
  return async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
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
      const formData = await request.formData();
      
      // Validate all files in the form data
      for (const [_key, value] of formData.entries()) {
        if (value instanceof File) {
          validateFile(value, options ?? {});
        }
      }

      const data = await handler(request, formData, session.user.token, safeParams);

      // Wrap the response in ApiResponse format if it's not already
      const response: ApiResponse<T> = typeof data === 'object' && data !== null && 'success' in data
        ? (data as unknown as ApiResponse<T>)
        : { success: true, message: "Success", data };

      return NextResponse.json(response);
    } catch (error) {
      return handleApiError(error);
    }
  };
}