// WCAG 2.x contrast of a text color against white, for tests that assert a
// rendered color stays readable. Accepts "#RRGGBB" or the "rgb(r, g, b)" form
// jsdom normalizes inline `style.color` to.
function channels(color: string): [number, number, number] {
  const hex = /^#([0-9a-f]{6})$/i.exec(color.trim());
  if (hex?.[1]) {
    const value = hex[1];
    return [0, 2, 4].map((i) => parseInt(value.slice(i, i + 2), 16)) as [
      number,
      number,
      number,
    ];
  }
  const rgb = /^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/.exec(color.trim());
  if (rgb) return [Number(rgb[1]), Number(rgb[2]), Number(rgb[3])];
  throw new Error(`unsupported color: "${color}"`);
}

function luminance(color: string): number {
  const [r, g, b] = channels(color).map((v) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  }) as [number, number, number];
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

export function contrastOnWhite(color: string): number {
  return 1.05 / (luminance(color) + 0.05);
}
