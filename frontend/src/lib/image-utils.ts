import imageCompression from "browser-image-compression";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ImageUtils" });

function safeJpegFilename(fileName: string): string {
  const originalName = fileName.replace(/\.[^/.]+$/, "");
  const safeBase =
    originalName
      .replace(/\.+/g, "-")
      .replace(/[^a-zA-Z0-9_-]/g, "-")
      .replace(/-+/g, "-")
      .replace(/^-|-$/g, "") || "avatar";
  return `${safeBase}.jpg`;
}

/**
 * Compress avatar image before upload
 *
 * Reduces file size from ~5MB to ~200KB while maintaining good quality.
 * Resizes to max 512x512px which is perfect for avatar display sizes.
 *
 * @param file - Original image file selected by user
 * @returns Compressed image file ready for upload
 *
 * @example
 * const compressed = await compressAvatar(selectedFile);
 * await uploadAvatar(compressed); // Upload is 25x faster!
 */
export async function compressAvatar(file: File): Promise<File> {
  const options = {
    maxSizeMB: 0.3, // Max 300KB (perfect balance of size/quality)
    maxWidthOrHeight: 512, // 512px is plenty for avatars (displayed at 32-128px)
    useWebWorker: true, // Uses Web Worker (no UI freeze during compression)
    fileType: "image/jpeg", // JPEG optimal for photos
  };

  try {
    const compressedBlob = await imageCompression(file, options);

    // Create new File with a single safe extension. The upload proxy rejects
    // multiple-dot filenames as suspicious, and compression always emits JPEG.
    const newFileName = safeJpegFilename(file.name);

    // Convert blob to File with proper name and type
    const compressedFile = new File([compressedBlob], newFileName, {
      type: "image/jpeg",
      lastModified: Date.now(),
    });

    return compressedFile;
  } catch (error) {
    logger.error("avatar compression failed, using original", {
      error: String(error),
    });
    // Fallback to original file if compression fails
    return file;
  }
}
