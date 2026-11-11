/**
 * Removes a plain background from a logo, so it sits cleanly on the white
 * plate of a QR code. Works on raw RGBA pixels and never leaves the device.
 *
 * Only a background that reaches the edge goes: the fill starts at the border
 * and stops at the first pixel of another colour. White inside a logo, such as
 * the counter of a letter, stays.
 */

export interface PixelImage {
  readonly data: Uint8ClampedArray;
  readonly width: number;
  readonly height: number;
}

type Rgb = readonly [number, number, number];

/** Colour distance up to which a pixel counts as background. */
const TOLERANCE = 36;
/** Up to here an edge pixel fades out instead of staying hard (anti-aliasing). */
const SOFT_TOLERANCE = 96;
/** Share of the border that must match the corners for a plain background. */
const BORDER_MATCH_SHARE = 0.7;
const OPAQUE = 250;

function distance(data: Uint8ClampedArray, offset: number, color: Rgb): number {
  const dr = data[offset]! - color[0];
  const dg = data[offset + 1]! - color[1];
  const db = data[offset + 2]! - color[2];
  return Math.sqrt(dr * dr + dg * dg + db * db);
}

function borderOffsets({ width, height }: PixelImage): number[] {
  const offsets: number[] = [];
  for (let x = 0; x < width; x += 1) {
    offsets.push(x * 4, ((height - 1) * width + x) * 4);
  }
  for (let y = 1; y < height - 1; y += 1) {
    offsets.push(y * width * 4, (y * width + width - 1) * 4);
  }
  return offsets;
}

/**
 * The colour of a plain, opaque background, or `null` when the image has none:
 * it is already cut out, or its edge is a photo.
 */
export function detectPlainBackground(image: PixelImage): Rgb | null {
  const { data, width, height } = image;
  if (width < 3 || height < 3) return null;

  const corners = [
    0,
    (width - 1) * 4,
    (height - 1) * width * 4,
    (height * width - 1) * 4,
  ];
  if (corners.some((offset) => data[offset + 3]! < OPAQUE)) return null;

  const color: Rgb = [0, 1, 2].map(
    (channel) =>
      corners.reduce((sum, offset) => sum + data[offset + channel]!, 0) /
      corners.length,
  ) as unknown as Rgb;
  if (corners.some((offset) => distance(data, offset, color) > TOLERANCE)) {
    return null;
  }

  const border = borderOffsets(image);
  const matching = border.filter(
    (offset) =>
      data[offset + 3]! >= OPAQUE && distance(data, offset, color) <= TOLERANCE,
  ).length;
  return matching / border.length >= BORDER_MATCH_SHARE ? color : null;
}

/**
 * Makes the background transparent, in place. Pixels on the rim between
 * background and logo keep a partial alpha so the edge stays smooth.
 */
export function removePlainBackground(image: PixelImage, color: Rgb): void {
  const { data, width, height } = image;
  const removed = new Uint8Array(width * height);
  const queue: number[] = [];

  const visit = (index: number) => {
    if (removed[index]) return;
    const offset = index * 4;
    if (data[offset + 3]! < OPAQUE) return;
    const d = distance(data, offset, color);
    if (d <= TOLERANCE) {
      removed[index] = 1;
      data[offset + 3] = 0;
      queue.push(index);
    } else if (d < SOFT_TOLERANCE) {
      // Rim pixel: a blend of logo and background. It fades, the fill stops.
      data[offset + 3] = Math.round(
        (255 * (d - TOLERANCE)) / (SOFT_TOLERANCE - TOLERANCE),
      );
    }
  };

  for (const offset of borderOffsets(image)) visit(offset / 4);
  while (queue.length > 0) {
    const index = queue.pop()!;
    const x = index % width;
    if (x > 0) visit(index - 1);
    if (x < width - 1) visit(index + 1);
    if (index >= width) visit(index - width);
    if (index < width * (height - 1)) visit(index + width);
  }
}

/** The smallest box around everything visible, or `null` for an empty image. */
export function visibleBounds(
  image: PixelImage,
): { x: number; y: number; width: number; height: number } | null {
  const { data, width, height } = image;
  let left = width;
  let top = height;
  let right = -1;
  let bottom = -1;
  for (let y = 0; y < height; y += 1) {
    for (let x = 0; x < width; x += 1) {
      if (data[(y * width + x) * 4 + 3]! <= 8) continue;
      if (x < left) left = x;
      if (x > right) right = x;
      if (y < top) top = y;
      if (y > bottom) bottom = y;
    }
  }
  if (right < 0) return null;
  return { x: left, y: top, width: right - left + 1, height: bottom - top + 1 };
}
