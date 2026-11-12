import {
  QR_DOT_RADIUS,
  QR_FINDER_RINGS,
  QR_INK,
  QR_PAPER,
  QR_PLATE_RADIUS,
  type BrandedQrLayout,
} from "./branded-qr";
import {
  detectPlainBackground,
  removePlainBackground,
  visibleBounds,
} from "./logo-background";

/** Browser side of the QR code: prepare the logo, draw the PNG. */

/** Longest side of the embedded logo. Enough for a poster, small in the SVG. */
const LOGO_MAX_SIDE = 640;

export interface QrLogo {
  /** PNG as a `data:` URL, cut to what is visible. */
  readonly dataUrl: string;
  /** Width divided by height. */
  readonly aspect: number;
  /** The picture has a plain background that can be removed. */
  readonly hasPlainBackground: boolean;
}

function canvasContext(width: number, height: number) {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (!context) throw new Error("canvas_unavailable");
  return { canvas, context };
}

/**
 * Turns a picture into the logo for the code: scaled down, its plain
 * background removed on request, and cut to what is visible so the logo fills
 * the plate.
 */
export async function prepareQrLogo(
  source: Blob | HTMLImageElement,
  options: { readonly removeBackground: boolean },
): Promise<QrLogo> {
  const bitmap = await createImageBitmap(source);
  try {
    const scale = Math.min(
      1,
      LOGO_MAX_SIDE / Math.max(bitmap.width, bitmap.height),
    );
    const width = Math.max(1, Math.round(bitmap.width * scale));
    const height = Math.max(1, Math.round(bitmap.height * scale));
    const { context } = canvasContext(width, height);
    context.drawImage(bitmap, 0, 0, width, height);
    const pixels = context.getImageData(0, 0, width, height);

    const background = detectPlainBackground(pixels);
    if (background && options.removeBackground) {
      removePlainBackground(pixels, background);
    }
    const bounds = visibleBounds(pixels) ?? { x: 0, y: 0, width, height };
    const cut = canvasContext(bounds.width, bounds.height);
    cut.context.putImageData(pixels, -bounds.x, -bounds.y);

    return {
      dataUrl: cut.canvas.toDataURL("image/png"),
      aspect: bounds.width / bounds.height,
      hasPlainBackground: background !== null,
    };
  } finally {
    bitmap.close();
  }
}

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("logo_decode_failed"));
    image.src = src;
  });
}

/**
 * Draws the code as a PNG. It paints the layout itself instead of rastering
 * the SVG: Safari skips an embedded logo the first time an SVG image is drawn.
 */
export async function brandedQrPng(
  layout: BrandedQrLayout,
  options: { readonly logoDataUrl?: string; readonly pixelSize: number },
): Promise<Blob> {
  const { canvas, context } = canvasContext(
    options.pixelSize,
    options.pixelSize,
  );
  const unit = options.pixelSize / layout.size;
  context.scale(unit, unit);
  context.imageSmoothingQuality = "high";

  context.fillStyle = QR_PAPER;
  context.fillRect(0, 0, layout.size, layout.size);

  context.fillStyle = QR_INK;
  context.beginPath();
  for (const { cx, cy } of layout.dots) {
    context.moveTo(cx + QR_DOT_RADIUS, cy);
    context.arc(cx, cy, QR_DOT_RADIUS, 0, Math.PI * 2);
  }
  context.fill();

  for (const finder of layout.finders) {
    for (const ring of QR_FINDER_RINGS) {
      context.fillStyle = ring.ink ? QR_INK : QR_PAPER;
      context.beginPath();
      context.roundRect(
        finder.x + ring.inset,
        finder.y + ring.inset,
        ring.side,
        ring.side,
        ring.radius,
      );
      context.fill();
    }
  }

  if (layout.plate) {
    context.fillStyle = QR_PAPER;
    context.beginPath();
    context.roundRect(
      layout.plate.x,
      layout.plate.y,
      layout.plate.width,
      layout.plate.height,
      QR_PLATE_RADIUS,
    );
    context.fill();
  }
  if (layout.logo && options.logoDataUrl) {
    const image = await loadImage(options.logoDataUrl);
    context.drawImage(
      image,
      layout.logo.x,
      layout.logo.y,
      layout.logo.width,
      layout.logo.height,
    );
  }

  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error("png_failed"))),
      "image/png",
    );
  });
}
