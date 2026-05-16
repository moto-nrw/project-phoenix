// Magic-byte image validation for upload route handlers.

interface ImageMagicSignature {
  // Lower-case hex of the first bytes; matched via startsWith against the
  // first 4 bytes (8 hex chars).
  readonly prefix: string;
  // Required for container formats whose 4-byte prefix isn't unique (RIFF
  // wraps WebP/WAV/AVI). Receives the first 12 bytes.
  readonly verifyExtras?: (bytes: Uint8Array) => boolean;
}

export const JPEG_SIGNATURE: ImageMagicSignature = { prefix: "ffd8ff" };

export const PNG_SIGNATURE: ImageMagicSignature = { prefix: "89504e47" };

export const WEBP_SIGNATURE: ImageMagicSignature = {
  prefix: "52494646",
  // RIFF + "WEBP" marker at offsets 8-11 — disambiguates from WAV/AVI.
  verifyExtras: (bytes) =>
    bytes[8] === 0x57 &&
    bytes[9] === 0x45 &&
    bytes[10] === 0x42 &&
    bytes[11] === 0x50,
};

export const GIF_SIGNATURE: ImageMagicSignature = { prefix: "47494638" };

export function validateImageMagicBytes(
  buffer: ArrayBuffer,
  signatures: readonly ImageMagicSignature[],
): boolean {
  const bytes = new Uint8Array(buffer).subarray(0, 12);
  const header = Array.from(bytes.subarray(0, 4))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
  for (const sig of signatures) {
    if (!header.startsWith(sig.prefix)) continue;
    if (sig.verifyExtras && !sig.verifyExtras(bytes)) continue;
    return true;
  }
  return false;
}
