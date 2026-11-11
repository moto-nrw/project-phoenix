import qrcode from "qrcode-generator";

/**
 * The moto QR code: round dots, rounded corner markers and a logo on a white
 * plate in the middle. One layout feeds the SVG and the PNG, so the preview,
 * both downloads and the decoding test draw the same code.
 *
 * Error correction is always H (30 %): the plate hides modules, and a code on
 * a printed notice has to survive a crease or a bad photo as well.
 */

/** Free modules around the code; the standard asks for four. */
const QUIET_ZONE = 4;
/** Side of a corner marker in modules. */
const FINDER_SIZE = 7;
/**
 * Share of the code's area the plate aims for and may at most hide, although
 * H recovers about 30 % of the code words. Measured with a solid black logo
 * and the jsQR reader: codes up to 57 modules (links up to about 120
 * characters) first fail at 12 %, larger ones at 9 %. The limits keep a
 * quarter below that for print damage (decoding test: branded-qr.test.ts).
 */
const PLATE_SHARE = { target: 0.08, max: 0.09 };
const DENSE_PLATE_SHARE = { target: 0.05, max: 0.06 };
const DENSE_FROM_MODULES = 61;
/** Wide enough for a word mark, and still clear of the corner markers. */
const PLATE_MAX_WIDTH_SHARE = 0.52;
const PLATE_MAX_HEIGHT_SHARE = 0.32;
/** Flatter or taller logos are fitted into a plate of these proportions. */
const MIN_ASPECT = 0.5;
const MAX_ASPECT = 7;
/** White rim between logo and plate edge, in modules. */
const PLATE_PADDING = 0.6;

export const QR_DOT_RADIUS = 0.48;
export const QR_INK = "black";
export const QR_PAPER = "white";

interface QrRect {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

export interface BrandedQrLayout {
  /** Side of the drawing in modules, quiet zone included. */
  readonly size: number;
  /** Centres of the round data modules. */
  readonly dots: readonly { readonly cx: number; readonly cy: number }[];
  /** Top-left corners of the three 7 × 7 corner markers. */
  readonly finders: readonly { readonly x: number; readonly y: number }[];
  /** White plate in the middle; `null` without a logo. */
  readonly plate: QrRect | null;
  /** Where the logo sits inside the plate. */
  readonly logo: QrRect | null;
}

/** The largest odd number of modules that is at most `value`, at least 3. */
function oddAtMost(value: number): number {
  const whole = Math.floor(value);
  return Math.max(3, whole % 2 === 0 ? whole - 1 : whole);
}

function plateFor(modules: number, aspect: number): QrRect {
  const ratio = Math.min(MAX_ASPECT, Math.max(MIN_ASPECT, aspect));
  const share = modules >= DENSE_FROM_MODULES ? DENSE_PLATE_SHARE : PLATE_SHARE;
  // The module count is odd, so an odd plate lands on the module grid.
  const width = oddAtMost(
    Math.min(
      Math.sqrt(share.target * ratio) * modules,
      PLATE_MAX_WIDTH_SHARE * modules,
    ),
  );
  // The height that holds the logo at full plate width, rounded up to the
  // grid while the plate stays inside its share; otherwise the logo shrinks.
  const needed = (width - 2 * PLATE_PADDING) / ratio + 2 * PLATE_PADDING;
  const snug = oddAtMost(needed);
  const roomy = snug < needed ? snug + 2 : snug;
  const fits =
    roomy <= PLATE_MAX_HEIGHT_SHARE * modules &&
    width * roomy <= share.max * modules * modules;
  const height = fits ? roomy : snug;
  return {
    x: QUIET_ZONE + (modules - width) / 2,
    y: QUIET_ZONE + (modules - height) / 2,
    width,
    height,
  };
}

function inside(rect: QrRect, x: number, y: number): boolean {
  return (
    x >= rect.x &&
    x < rect.x + rect.width &&
    y >= rect.y &&
    y < rect.y + rect.height
  );
}

/**
 * Lays out the code for `value`. `logoAspect` is the logo's width divided by
 * its height; without it the code has no plate.
 */
export function brandedQrLayout(
  value: string,
  logoAspect?: number,
): BrandedQrLayout {
  const code = qrcode(0, "H");
  code.addData(value);
  code.make();
  const modules = code.getModuleCount();

  const finders = [
    { x: QUIET_ZONE, y: QUIET_ZONE },
    { x: QUIET_ZONE + modules - FINDER_SIZE, y: QUIET_ZONE },
    { x: QUIET_ZONE, y: QUIET_ZONE + modules - FINDER_SIZE },
  ];
  const finderRects = finders.map((finder) => ({
    ...finder,
    width: FINDER_SIZE,
    height: FINDER_SIZE,
  }));
  const plate =
    logoAspect !== undefined && logoAspect > 0
      ? plateFor(modules, logoAspect)
      : null;

  const dots: { cx: number; cy: number }[] = [];
  for (let row = 0; row < modules; row += 1) {
    for (let col = 0; col < modules; col += 1) {
      if (!code.isDark(row, col)) continue;
      const x = QUIET_ZONE + col;
      const y = QUIET_ZONE + row;
      if (finderRects.some((rect) => inside(rect, x, y))) continue;
      if (plate && inside(plate, x, y)) continue;
      dots.push({ cx: x + 0.5, cy: y + 0.5 });
    }
  }

  let logo: QrRect | null = null;
  if (plate && logoAspect !== undefined) {
    const maxWidth = plate.width - 2 * PLATE_PADDING;
    const maxHeight = plate.height - 2 * PLATE_PADDING;
    const width = Math.min(maxWidth, maxHeight * logoAspect);
    const height = Math.min(maxHeight, width / logoAspect);
    logo = {
      x: plate.x + (plate.width - width) / 2,
      y: plate.y + (plate.height - height) / 2,
      width,
      height,
    };
  }

  return { size: modules + 2 * QUIET_ZONE, dots, finders, plate, logo };
}

/** The three nested squares of a corner marker: offset, side, corner radius, ink. */
export const QR_FINDER_RINGS = [
  { inset: 0, side: 7, radius: 1.6, ink: true },
  { inset: 1, side: 5, radius: 1, ink: false },
  { inset: 2, side: 3, radius: 0.7, ink: true },
] as const;

export const QR_PLATE_RADIUS = 0.9;

function escapeAttribute(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function round(value: number): string {
  return String(Math.round(value * 1000) / 1000);
}

/**
 * The code as a standalone SVG. `logoDataUrl` must be a `data:` URL so the
 * file carries its logo with it.
 */
export function brandedQrSvg(
  layout: BrandedQrLayout,
  options: {
    readonly label: string;
    readonly logoDataUrl?: string;
    /** Printed width and height in pixels; the drawing scales freely. */
    readonly pixelSize?: number;
  },
): string {
  const { size } = layout;
  const pixelSize = options.pixelSize ?? 1024;
  const r = QR_DOT_RADIUS;
  const dots = layout.dots
    .map(
      ({ cx, cy }) =>
        `M${round(cx - r)} ${round(cy)}a${r} ${r} 0 1 0 ${r * 2} 0a${r} ${r} 0 1 0 ${-r * 2} 0z`,
    )
    .join("");
  const finders = layout.finders
    .flatMap((finder) =>
      QR_FINDER_RINGS.map(
        (ring) =>
          `<rect x="${finder.x + ring.inset}" y="${finder.y + ring.inset}" width="${ring.side}" height="${ring.side}" rx="${ring.radius}" fill="${ring.ink ? QR_INK : QR_PAPER}"/>`,
      ),
    )
    .join("");
  const plate = layout.plate
    ? `<rect x="${round(layout.plate.x)}" y="${round(layout.plate.y)}" width="${layout.plate.width}" height="${layout.plate.height}" rx="${QR_PLATE_RADIUS}" fill="${QR_PAPER}"/>`
    : "";
  const logo =
    layout.logo && options.logoDataUrl
      ? `<image href="${escapeAttribute(options.logoDataUrl)}" x="${round(layout.logo.x)}" y="${round(layout.logo.y)}" width="${round(layout.logo.width)}" height="${round(layout.logo.height)}" preserveAspectRatio="xMidYMid meet"/>`
      : "";

  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${size} ${size}" width="${pixelSize}" height="${pixelSize}" role="img" aria-label="${escapeAttribute(options.label)}">` +
    `<rect width="${size}" height="${size}" fill="${QR_PAPER}"/>` +
    `<path fill="${QR_INK}" d="${dots}"/>` +
    finders +
    plate +
    logo +
    `</svg>`
  );
}
