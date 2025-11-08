import imageCompression from "browser-image-compression";

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

    // Create new File with proper filename and extension
    // Preserve original filename but change extension to .jpg (since we convert to JPEG)
    const originalName = file.name.replace(/\.[^/.]+$/, ""); // Remove old extension
    const newFileName = `${originalName}.jpg`;

    // Convert blob to File with proper name and type
    const compressedFile = new File([compressedBlob], newFileName, {
      type: "image/jpeg",
      lastModified: Date.now(),
    });

    return compressedFile;
  } catch (error) {
    console.error("Avatar compression failed, using original:", error);
    // Fallback to original file if compression fails
    return file;
  }
}
