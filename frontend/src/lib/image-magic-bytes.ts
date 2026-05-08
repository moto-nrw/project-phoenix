// Server-side image magic-byte validation.
//
// Used by Next.js route handlers that proxy file uploads to the backend
// to reject inputs whose declared MIME type doesn't match the actual
// header bytes — a `.jpg` extension on a renamed `.exe` should fail at
// the proxy boundary, not at the backend.
//
// Pure ES (no DOM, no Node-only APIs), so it runs in both runtime
// targets: the standard Next.js server route handler and any future Edge
// usage. The frontend's browser-side image compression flow doesn't need
// this — `browser-image-compression` decodes via canvas and would refuse
// non-image bytes itself.

interface ImageMagicSignature {
  /**
   * Lower-case hex string of the expected first bytes. The validator
   * compares this against the first 4 bytes of the file (8 hex chars);
   * shorter prefixes match by `String.startsWith`.
   *
   * Examples:
   *   - "ffd8ff"   — JPEG (SOI marker; final byte varies by encoder)
   *   - "89504e47" — PNG (\x89 P N G)
   *   - "52494646" — RIFF container (must combine with verifyExtras)
   *   - "47494638" — GIF8(7|9)a
   */
  readonly prefix: string;
  /**
   * Optional secondary check. Required for container formats where the
   * 4-byte prefix isn't unique (e.g. RIFF wraps WebP, WAV, AVI — the
   * "WEBP" marker at offset 8-11 is what disambiguates WebP).
   * Receives the first 12 bytes of the file.
   */
  readonly verifyExtras?: (bytes: Uint8Array) => boolean;
}

export const JPEG_SIGNATURE: ImageMagicSignature = { prefix: "ffd8ff" };

export const PNG_SIGNATURE: ImageMagicSignature = { prefix: "89504e47" };

export const WEBP_SIGNATURE: ImageMagicSignature = {
  prefix: "52494646",
  // RIFF container — verify the WEBP marker at offsets 8-11 (W=0x57,
  // E=0x45, B=0x42, P=0x50). Without this check we'd green-light WAV/AVI.
  verifyExtras: (bytes) =>
    bytes[8] === 0x57 &&
    bytes[9] === 0x45 &&
    bytes[10] === 0x42 &&
    bytes[11] === 0x50,
};

export const GIF_SIGNATURE: ImageMagicSignature = { prefix: "47494638" };

/**
 * Returns true when the buffer starts with bytes matching one of the
 * provided signatures. Reads only the first 12 bytes — every supported
 * format's discriminator fits in that window — so passing a 5 MiB
 * ArrayBuffer is fine.
 */
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
