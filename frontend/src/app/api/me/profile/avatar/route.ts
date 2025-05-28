import type { NextRequest } from "next/server";
import { createFileUploadHandler } from "~/lib/file-upload-wrapper";
import { createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendProfile } from "~/lib/profile-helpers";

interface ProfileResponse {
  success: boolean;
  message: string;
  data: BackendProfile;
}

// POST handler for avatar upload with security validation
export const POST = createFileUploadHandler<BackendProfile>(
  async (request: NextRequest, formData: FormData, token: string) => {
    // Get the avatar file from form data
    const avatarFile = formData.get('avatar');
    if (!avatarFile || !(avatarFile instanceof File)) {
      throw new Error("No avatar file provided");
    }

    // Additional validation: Check file header/magic bytes for actual file type
    // This helps prevent malicious files disguised with false extensions
    const buffer = await avatarFile.arrayBuffer();
    const bytes = new Uint8Array(buffer).subarray(0, 12); // Get more bytes for better detection
    const header = Array.from(bytes.subarray(0, 4))
      .map(byte => byte.toString(16).padStart(2, '0'))
      .join('');
    
    // Common image file signatures
    const validHeaders: Record<string, string[]> = {
      'ffd8ff': ['image/jpeg', 'image/jpg'],  // JPEG
      '89504e47': ['image/png'],              // PNG
      '47494638': ['image/gif'],              // GIF
      '52494646': ['image/webp'],             // WebP (RIFF header)
    };

    let isValidHeader = false;
    let detectedType = '';
    
    for (const [signature, types] of Object.entries(validHeaders)) {
      if (header.startsWith(signature)) {
        isValidHeader = true;
        detectedType = types[0] ?? '';
        break;
      }
      // Special check for WebP (RIFF....WEBP)
      if (signature === '52494646' && header.startsWith('52494646') && 
          bytes[8] === 0x57 && bytes[9] === 0x45 && bytes[10] === 0x42 && bytes[11] === 0x50) {
        isValidHeader = true;
        detectedType = types[0] ?? '';
        break;
      }
    }

    if (!isValidHeader) {
      throw new Error("Invalid file format detected. Please upload a valid image file.");
    }

    // Verify the detected type matches the declared MIME type
    if (detectedType && avatarFile.type !== detectedType) {
      // Allow some flexibility for JPEG variants
      const isJpegVariant = (detectedType === 'image/jpeg' || detectedType === 'image/jpg') && 
                           (avatarFile.type === 'image/jpeg' || avatarFile.type === 'image/jpg');
      
      if (!isJpegVariant) {
        console.warn(`File type mismatch: declared=${avatarFile.type}, detected=${detectedType}`);
        // Don't throw error, just log warning - let backend handle final validation
      }
    }

    // Create new FormData with the validated file
    // We need to create a new file from the buffer since we consumed the stream
    const validatedFile = new File([buffer], avatarFile.name, { type: avatarFile.type });
    const validatedFormData = new FormData();
    validatedFormData.append('avatar', validatedFile);

    // Forward the request to backend
    const backendUrl = `${process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'}/api/me/profile/avatar`;
    const response = await fetch(backendUrl, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
      body: validatedFormData,
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(errorText || `Upload failed with status ${response.status}`);
    }

    const responseData = await response.json() as ProfileResponse;
    
    // Return the backend profile data (will be wrapped in ApiResponse by the handler)
    return responseData.data;
  },
  {
    maxSizeInMB: 5,
    allowedMimeTypes: ['image/jpeg', 'image/jpg', 'image/png', 'image/gif', 'image/webp'],
    allowedExtensions: ['.jpg', '.jpeg', '.png', '.gif', '.webp']
  }
);

// DELETE handler for removing avatar
export const DELETE = createDeleteHandler(async (request: NextRequest, token: string) => {
  // Forward the request to backend
  const backendUrl = `${process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080'}/api/me/profile/avatar`;
  const response = await fetch(backendUrl, {
    method: 'DELETE',
    headers: {
      'Authorization': `Bearer ${token}`,
    },
  });

  if (!response.ok) {
    const errorText = await response.text();
    throw new Error(errorText || `Delete failed with status ${response.status}`);
  }

  const responseData = await response.json() as ProfileResponse;
  
  // Return the updated profile data
  return responseData.data;
});