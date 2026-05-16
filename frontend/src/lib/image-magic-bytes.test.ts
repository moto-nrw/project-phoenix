// Coverage for the magic-byte validator. The proxy uses this to reject
// renamed-binary uploads where the declared MIME type lies about content
// (e.g. an `.exe` renamed to `.jpg`). The branches below pin both the
// happy paths for each accepted format and the rejection path for the
// RIFF/WEBP secondary verifier — without that secondary check WAV/AVI
// files would slip through under a `.webp` extension.

import { describe, it, expect } from "vitest";
import {
  JPEG_SIGNATURE,
  PNG_SIGNATURE,
  WEBP_SIGNATURE,
  GIF_SIGNATURE,
  validateImageMagicBytes,
} from "./image-magic-bytes";

function bufferOf(bytes: number[]): ArrayBuffer {
  return new Uint8Array(bytes).buffer;
}

describe("validateImageMagicBytes", () => {
  it("accepts a JPEG SOI marker", () => {
    const buf = bufferOf([0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0, 0, 0, 0, 0]);
    expect(validateImageMagicBytes(buf, [JPEG_SIGNATURE])).toBe(true);
  });

  it("accepts a PNG signature", () => {
    const buf = bufferOf([0x89, 0x50, 0x4e, 0x47, 0, 0, 0, 0, 0, 0, 0, 0]);
    expect(validateImageMagicBytes(buf, [PNG_SIGNATURE])).toBe(true);
  });

  it("accepts a RIFF container with the WEBP marker", () => {
    // 0x52 0x49 0x46 0x46 = "RIFF"; bytes 8-11 = "WEBP".
    const buf = bufferOf([
      0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
    ]);
    expect(validateImageMagicBytes(buf, [WEBP_SIGNATURE])).toBe(true);
  });

  it("rejects a RIFF container that is NOT WebP (e.g. WAVE)", () => {
    // RIFF prefix matches but bytes 8-11 are "WAVE" — verifyExtras must reject.
    const buf = bufferOf([
      0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x41, 0x56, 0x45,
    ]);
    expect(validateImageMagicBytes(buf, [WEBP_SIGNATURE])).toBe(false);
  });

  it("rejects bytes that do not match any provided signature", () => {
    // Random non-image bytes — no signature matches.
    const buf = bufferOf([0x00, 0x01, 0x02, 0x03, 0, 0, 0, 0, 0, 0, 0, 0]);
    expect(
      validateImageMagicBytes(buf, [
        JPEG_SIGNATURE,
        PNG_SIGNATURE,
        WEBP_SIGNATURE,
        GIF_SIGNATURE,
      ]),
    ).toBe(false);
  });

  it("returns false when signatures list is empty", () => {
    const buf = bufferOf([0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0, 0, 0, 0, 0]);
    expect(validateImageMagicBytes(buf, [])).toBe(false);
  });

  it("accepts a GIF signature when included in the allow-list", () => {
    const buf = bufferOf([0x47, 0x49, 0x46, 0x38, 0, 0, 0, 0, 0, 0, 0, 0]);
    expect(validateImageMagicBytes(buf, [GIF_SIGNATURE])).toBe(true);
  });
});
