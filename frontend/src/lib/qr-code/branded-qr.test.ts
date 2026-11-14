import jsQR from "jsqr";
import { describe, expect, it } from "vitest";
import {
  brandedQrLayout,
  brandedQrSvg,
  QR_DOT_RADIUS,
  QR_FINDER_RINGS,
  type BrandedQrLayout,
} from "./branded-qr";

const SCALE = 8;

/**
 * Paints the layout the way the SVG and the PNG do (round dots, corner
 * markers, white plate) and fills the logo area solid black: the worst logo a
 * reader can meet.
 */
function rasterize(layout: BrandedQrLayout): {
  data: Uint8ClampedArray;
  side: number;
} {
  const side = layout.size * SCALE;
  const dark = new Uint8Array(side * side);
  const fill = (
    x: number,
    y: number,
    width: number,
    height: number,
    value: number,
  ) => {
    for (let py = Math.round(y * SCALE); py < (y + height) * SCALE; py += 1) {
      for (let px = Math.round(x * SCALE); px < (x + width) * SCALE; px += 1) {
        dark[py * side + px] = value;
      }
    }
  };
  for (const { cx, cy } of layout.dots) {
    for (let py = (cy - 0.5) * SCALE; py < (cy + 0.5) * SCALE; py += 1) {
      for (let px = (cx - 0.5) * SCALE; px < (cx + 0.5) * SCALE; px += 1) {
        const dx = (px + 0.5) / SCALE - cx;
        const dy = (py + 0.5) / SCALE - cy;
        if (dx * dx + dy * dy <= QR_DOT_RADIUS * QR_DOT_RADIUS) {
          dark[py * side + px] = 1;
        }
      }
    }
  }
  for (const finder of layout.finders) {
    for (const ring of QR_FINDER_RINGS) {
      fill(
        finder.x + ring.inset,
        finder.y + ring.inset,
        ring.side,
        ring.side,
        ring.ink ? 1 : 0,
      );
    }
  }
  if (layout.plate) {
    const { x, y, width, height } = layout.plate;
    fill(x, y, width, height, 0);
  }
  if (layout.logo) {
    const { x, y, width, height } = layout.logo;
    fill(x, y, width, height, 1);
  }

  const data = new Uint8ClampedArray(side * side * 4);
  for (let i = 0; i < dark.length; i += 1) {
    const value = dark[i] ? 0 : 255;
    data.set([value, value, value, 255], i * 4);
  }
  return { data, side };
}

function decode(layout: BrandedQrLayout): string | undefined {
  const { data, side } = rasterize(layout);
  return jsQR(data, side, side)?.data;
}

const LINKS = [
  // The shortest link moto hands out: the smallest code, the tightest plate.
  "https://moto-app.de/anmelden",
  "https://ogs-musterschule.moto-app.de/anmeldung",
  "https://ogs-musterschule.moto-app.de/anmeldung?phase=4821",
  "https://eltern.moto-app.de/ferienanmeldung/3f9c1b7e5a2d4c6f8a0b1c2d3e4f5a6b7c8d9e0f",
  // 69 modules: the size that tolerated the smallest logo when measured.
  `https://moto-app.de/${"k7Q2x9".repeat(24)}k7Q2x`,
  // A one-time display link carries a long key.
  `https://ogs-musterschule.moto-app.de/display/${"k7Q2x".repeat(24)}`,
];
/** Wide word mark, square emblem, tall crest, and the clamped extremes. */
const LOGO_ASPECTS = [undefined, 6.5, 3, 1.45, 1, 0.6, 12, 0.1];

describe("brandedQrLayout", () => {
  it.each(LINKS)("stays readable with every logo shape: %s", (link) => {
    for (const aspect of LOGO_ASPECTS) {
      expect(decode(brandedQrLayout(link, aspect)), `aspect ${aspect}`).toBe(
        link,
      );
    }
  });

  it("has no plate without a logo", () => {
    const layout = brandedQrLayout(LINKS[1]!);

    expect(layout.plate).toBeNull();
    expect(layout.logo).toBeNull();
  });

  it("centres the plate on the module grid and keeps dots out of it", () => {
    const layout = brandedQrLayout(LINKS[1]!, 3);
    const plate = layout.plate!;

    expect(Number.isInteger(plate.x)).toBe(true);
    expect(Number.isInteger(plate.y)).toBe(true);
    expect(plate.x + plate.width / 2).toBe(layout.size / 2);
    expect(plate.y + plate.height / 2).toBe(layout.size / 2);
    expect(
      layout.dots.filter(
        ({ cx, cy }) =>
          cx > plate.x &&
          cx < plate.x + plate.width &&
          cy > plate.y &&
          cy < plate.y + plate.height,
      ),
    ).toEqual([]);
    expect(layout.dots.length).toBeGreaterThan(100);
  });

  it("fits the logo into the plate with its proportions", () => {
    const layout = brandedQrLayout(LINKS[1]!, 2);
    const plate = layout.plate!;
    const logo = layout.logo!;

    expect(logo.width / logo.height).toBeCloseTo(2);
    expect(logo.x).toBeGreaterThan(plate.x);
    expect(logo.y).toBeGreaterThan(plate.y);
    expect(logo.x + logo.width).toBeLessThan(plate.x + plate.width);
    expect(logo.y + logo.height).toBeLessThan(plate.y + plate.height);
  });
});

describe("brandedQrSvg", () => {
  it("embeds the logo and escapes the label", () => {
    const layout = brandedQrLayout(LINKS[1]!, 1);
    const svg = brandedQrSvg(layout, {
      label: 'QR-Code "Anmeldung" <Eltern> & Kinder',
      logoDataUrl: "data:image/png;base64,AAAA",
    });

    expect(svg).toContain('href="data:image/png;base64,AAAA"');
    expect(svg).toContain(
      'aria-label="QR-Code &quot;Anmeldung&quot; &lt;Eltern&gt; &amp; Kinder"',
    );
    expect(svg).toContain(`viewBox="0 0 ${layout.size} ${layout.size}"`);
    // Three corner markers of three squares each, the paper and the plate.
    expect(svg.match(/<rect /g)).toHaveLength(11);
  });

  it("draws no image without a logo", () => {
    const svg = brandedQrSvg(brandedQrLayout(LINKS[1]!), { label: "QR-Code" });

    expect(svg).toContain("<path ");
    expect(svg.includes("<image")).toBe(false);
  });
});
