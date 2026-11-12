import { describe, expect, it } from "vitest";
import {
  detectPlainBackground,
  removePlainBackground,
  visibleBounds,
  type PixelImage,
} from "./logo-background";

type Rgba = readonly [number, number, number, number];

const WHITE: Rgba = [255, 255, 255, 255];
const RED: Rgba = [220, 38, 38, 255];
const CLEAR: Rgba = [0, 0, 0, 0];

/** Builds an image from rows of single-character pixels. */
function image(rows: readonly string[], palette: Record<string, Rgba>) {
  const width = rows[0]!.length;
  const data = new Uint8ClampedArray(width * rows.length * 4);
  rows.forEach((row, y) => {
    [...row].forEach((key, x) => data.set(palette[key]!, (y * width + x) * 4));
  });
  return { data, width, height: rows.length } satisfies PixelImage;
}

function alphaRows(img: PixelImage): string[] {
  const rows: string[] = [];
  for (let y = 0; y < img.height; y += 1) {
    let row = "";
    for (let x = 0; x < img.width; x += 1) {
      row += img.data[(y * img.width + x) * 4 + 3]! > 0 ? "#" : ".";
    }
    rows.push(row);
  }
  return rows;
}

const PALETTE = {
  w: WHITE,
  r: RED,
  ".": CLEAR,
  g: [244, 246, 248, 255],
} as const;

describe("detectPlainBackground", () => {
  it("finds a plain background, also a slightly uneven one", () => {
    const img = image(["wwgww", "wrrrw", "wrwrw", "wrrrw", "wwwwg"], PALETTE);

    const color = detectPlainBackground(img);

    expect(color?.map(Math.round)).toEqual([252, 253, 253]);
  });

  it("leaves a logo that is already cut out", () => {
    const img = image([".....", ".rrr.", ".rrr.", "....."], PALETTE);

    expect(detectPlainBackground(img)).toBeNull();
  });

  it("leaves a picture whose edge is not one colour", () => {
    const img = image(["wrwrw", "rwrwr", "wrwrw", "rwrwr", "wrwrw"], PALETTE);

    expect(detectPlainBackground(img)).toBeNull();
  });
});

describe("removePlainBackground", () => {
  it("clears the background from the edge and keeps white inside the logo", () => {
    const img = image(
      ["wwwwwww", "wrrrrrw", "wrwwwrw", "wrwwwrw", "wrrrrrw", "wwwwwww"],
      PALETTE,
    );

    removePlainBackground(img, detectPlainBackground(img)!);

    expect(alphaRows(img)).toEqual([
      ".......",
      ".#####.",
      ".#####.",
      ".#####.",
      ".#####.",
      ".......",
    ]);
  });

  it("fades the blended rim between logo and background", () => {
    const blend: Rgba = [240, 200, 200, 255];
    const img = image(["wwwww", "wbrbw", "wwwww"], { ...PALETTE, b: blend });

    removePlainBackground(img, [255, 255, 255]);

    const alpha = (x: number) => img.data[(img.width + x) * 4 + 3]!;
    expect(alpha(1)).toBeGreaterThan(0);
    expect(alpha(1)).toBeLessThan(255);
    expect(alpha(2)).toBe(255);
  });
});

describe("visibleBounds", () => {
  it("boxes what is visible", () => {
    const img = image(["......", "..rr..", "..rrr.", "......"], PALETTE);

    expect(visibleBounds(img)).toEqual({ x: 2, y: 1, width: 3, height: 2 });
  });

  it("reports an empty image", () => {
    expect(visibleBounds(image(["...", "..."], PALETTE))).toBeNull();
  });
});
