import { readFile } from "node:fs/promises";
import { join } from "node:path";
import sharp from "sharp";

import type { DeviceSpec } from "./devices";

// Rahmung (#3759): sharp setzt das Rohbild in den Bildschirmbereich des
// offiziellen, unveränderten Apple Product Bezels. Der Hintergrund bleibt
// transparent; nur das Gerät ist opak.

/** Web-Größen, in denen jedes Bild zusätzlich als WebP entsteht. */
export const WEBP_WIDTHS = [640, 960, 1280, 1600, 2400, 3200] as const;

export const BEZEL_DIR = join(import.meta.dirname, "bezels");

/**
 * Rahmt ein Rohbild. Das Rohbild liegt unter dem Bezel: dessen Bildschirm ist
 * transparent, seine Ränder und Rundungen decken das Bild ab, und außerhalb des
 * Geräts bleibt alles durchsichtig.
 */
export async function frameMockup(
  raw: Buffer,
  device: DeviceSpec,
  bezelDir: string = BEZEL_DIR,
): Promise<Buffer> {
  const { file, width, height, screen } = device.bezel;
  const bezel = await readFile(join(bezelDir, file));
  const meta = await sharp(bezel).metadata();
  if (meta.width !== width || meta.height !== height) {
    throw new Error(
      `Bezel ${file} misst ${meta.width}×${meta.height}, erwartet ${width}×${height}. Bezel und Bildschirmrechteck in devices.ts gehören zusammen.`,
    );
  }
  const screenshot = await sharp(raw)
    .resize(screen.width, screen.height, { fit: "fill" })
    .toBuffer();
  const composed = await sharp({
    create: {
      width,
      height,
      channels: 4,
      background: { r: 0, g: 0, b: 0, alpha: 0 },
    },
  })
    .composite([
      { input: screenshot, left: screen.left, top: screen.top },
      { input: bezel, left: 0, top: 0 },
    ])
    .raw()
    .toBuffer();

  // Die Ecken des rechteckigen Rohbilds liegen außerhalb der abgerundeten
  // Gerätekontur, wo der Bezel transparent ist. Dort darf nichts stehen
  // bleiben: alles, was vom Rand aus über transparente Bezel-Pixel erreichbar
  // ist, wird wieder durchsichtig.
  const bezelAlpha = await sharp(bezel)
    .ensureAlpha()
    .extractChannel(3)
    .raw()
    .toBuffer();
  const outside = outsideMask(bezelAlpha, width, height);
  for (let index = 0; index < outside.length; index += 1) {
    if (outside[index]) composed.fill(0, index * 4, index * 4 + 4);
  }
  return sharp(composed, { raw: { width, height, channels: 4 } })
    .png()
    .toBuffer();
}

/** Vom Bildrand aus erreichbare, vollständig transparente Pixel (Flood-Fill). */
function outsideMask(alpha: Buffer, width: number, height: number): Uint8Array {
  const mask = new Uint8Array(width * height);
  const stack: number[] = [];
  const push = (x: number, y: number) => {
    const index = y * width + x;
    if (mask[index] === 0 && alpha[index] === 0) {
      mask[index] = 1;
      stack.push(index);
    }
  };
  for (let x = 0; x < width; x += 1) {
    push(x, 0);
    push(x, height - 1);
  }
  for (let y = 0; y < height; y += 1) {
    push(0, y);
    push(width - 1, y);
  }
  while (stack.length > 0) {
    const index = stack.pop()!;
    const x = index % width;
    const y = (index - x) / width;
    if (x > 0) push(x - 1, y);
    if (x < width - 1) push(x + 1, y);
    if (y > 0) push(x, y - 1);
    if (y < height - 1) push(x, y + 1);
  }
  return mask;
}

export interface WebpVariant {
  readonly width: number;
  readonly height: number;
  readonly data: Buffer;
  /** Das Original ist schmaler als die Zielbreite: das Bild wurde vergrößert. */
  readonly upscaled: boolean;
}

/**
 * WebP-Größen eines PNG. Alle sechs Breiten entstehen immer, damit ein `srcSet`
 * nie auf eine fehlende Datei zeigt; ist das Original schmaler, wird es
 * vergrößert und in der Manifest-Zeile markiert.
 */
export async function webpVariants(png: Buffer): Promise<WebpVariant[]> {
  const sourceWidth = (await sharp(png).metadata()).width ?? 0;
  return Promise.all(
    WEBP_WIDTHS.map(async (width) => {
      const { data, info } = await sharp(png)
        .resize({ width, kernel: "lanczos3" })
        .webp({ quality: 92, alphaQuality: 100, effort: 5 })
        .toBuffer({ resolveWithObject: true });
      return {
        width: info.width,
        height: info.height,
        data,
        upscaled: width > sourceWidth,
      };
    }),
  );
}
