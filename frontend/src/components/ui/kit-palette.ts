/**
 * The kit's colour source, free of domain meaning (#6573). Every tint table in
 * `components/ui/` derives from the palette below: the duotone pairs, the pill
 * tones, the tinted surface classes and the text-safe shades. What a colour
 * MEANS (Schulhof, Krank, Warnung) lives in `LOCATION_COLORS`
 * (`lib/location-helper.ts`), which takes its values from here.
 *
 * The same values exist as `--color-moto-*` custom properties in globals.css,
 * which the `bg-moto-*` / `text-moto-*` utilities compile from.
 * `styles/moto-token-drift.test.ts` keeps both copies equal.
 */
export const MOTO_COLOR_PALETTE = {
  green: {
    soft: "#EEF9E1",
    muted: "#D7E8C3",
    light: "#92D63C",
    base: "#83CD2D",
    vivid: "#5F9F1B",
    hover: "#74B825",
    active: "#6DB118",
    strong: "#3F6F12",
    // Opaque green surface with white text or icon (#5141): fill 4.84:1,
    // fillHover 6.0:1 (= strong), fillActive 7.0:1. The base steps above do
    // not carry white (#83CD2D: 1.96:1).
    fill: "#4A7F14",
    fillHover: "#3F6F12",
    fillActive: "#386410",
  },
  blue: {
    soft: "#EDF3FC",
    light: "#6B95E0",
    base: "#5080D8",
    hover: "#3B68C0",
    strong: "#315C9B",
  },
  timeTracking: {
    soft: "#E0F2FE",
    light: "#7DD3FC",
    base: "#0EA5E9",
    strong: "#0369A1",
  },
  orange: {
    soft: "#FFF3E5",
    base: "#F78C10",
    hover: "#E07400",
    strong: "#9B5609",
  },
  red: {
    soft: "#FEF2F2",
    base: "#DC2626",
    hover: "#B91C1C",
    strong: "#B91C1C",
  },
  teal: {
    soft: "#E8F8F5",
    light: "#5CC8BA",
    base: "#159E90",
    strong: "#0F766E",
  },
  amber: {
    soft: "#FEF3C7",
    light: "#FACC15",
    base: "#EAB308",
    strong: "#92400E",
    // The "Achtung" pill (#5617). soft/strong read almost exactly like the
    // orange Schulhof pill (#FFF3E5 / #9B5609), so the pill takes a clearly
    // yellow tint and a darker label instead: 8.1:1.
    pill: "#FEF9C3",
    pillText: "#713F12",
  },
  purple: {
    soft: "#F3E8FF",
    light: "#A78BFA",
    base: "#7C3AED",
    strong: "#6B21A8",
  },
  magenta: {
    soft: "#FAE8FF",
    light: "#E879F9",
    base: "#D946EF",
    strong: "#86198F",
  },
  indigo: {
    soft: "#EEF2FF",
    light: "#818CF8",
    base: "#4F46E5",
    strong: "#3730A3",
  },
  coral: {
    soft: "#FFF0ED",
    light: "#F29A8D",
    base: "#E85D4A",
    strong: "#A83A2E",
  },
  cyan: {
    soft: "#ECFEFF",
    light: "#67E8F9",
    base: "#0891B2",
    strong: "#155E75",
  },
  navy: {
    soft: "#EEF4F8",
    light: "#7FA6C9",
    base: "#365D83",
    strong: "#1E3A5F",
  },
  mint: {
    soft: "#EAF9F3",
    light: "#8AD9BB",
    base: "#3BAF83",
    strong: "#187255",
  },
  wine: {
    soft: "#FBECEF",
    light: "#CF7180",
    base: "#8F2535",
    strong: "#681A27",
  },
  gold: {
    soft: "#FFF7E6",
    light: "#E6B85C",
    base: "#B7791F",
    strong: "#7A4A0B",
  },
  petrol: {
    soft: "#E9F7F6",
    light: "#72BDB8",
    base: "#217A78",
    strong: "#155A59",
  },
  neutral: {
    soft: "#F3F4F6",
    muted: "#D1D5DB",
    light: "#9CA3AF",
    base: "#6B7280",
    strong: "#374151",
    // gray-600, deliberately lighter than strong (gray-700): the neutral pill
    // should read quieter than a status label.
    pillText: "#4B5563",
  },
  stone: {
    soft: "#F5F5F4",
    light: "#A8A29E",
    base: "#78716C",
    strong: "#44403C",
  },
  // Accent hues without a ramp, only ever drawn as a fill (room categories).
  emerald: { base: "#10B981" },
  violet: { base: "#8B5CF6" },
  pink: { base: "#EC4899" },
} as const;

export type MotoPaletteHue = keyof typeof MOTO_COLOR_PALETTE;

// =============================================================================
// Duotone icons
// =============================================================================

export type MotoDuotoneTone =
  | "blue"
  | "timeTracking"
  | "teal"
  | "magenta"
  | "orange"
  | "amber"
  | "purple"
  | "greenDeep"
  | "greenVivid"
  | "indigo"
  | "red"
  | "coral"
  | "cyan"
  | "navy"
  | "mint"
  | "wine"
  | "gold"
  | "petrol"
  | "neutral"
  | "stone";

/** Primary and secondary layer of a duotone icon. */
export const MOTO_DUOTONE_TONES: Record<
  MotoDuotoneTone,
  { primary: string; secondary: string }
> = {
  blue: {
    primary: MOTO_COLOR_PALETTE.blue.strong,
    secondary: MOTO_COLOR_PALETTE.blue.light,
  },
  timeTracking: {
    primary: MOTO_COLOR_PALETTE.timeTracking.base,
    secondary: MOTO_COLOR_PALETTE.timeTracking.light,
  },
  teal: {
    primary: MOTO_COLOR_PALETTE.teal.strong,
    secondary: MOTO_COLOR_PALETTE.teal.light,
  },
  magenta: {
    primary: MOTO_COLOR_PALETTE.magenta.strong,
    secondary: MOTO_COLOR_PALETTE.magenta.light,
  },
  orange: {
    primary: MOTO_COLOR_PALETTE.orange.strong,
    secondary: MOTO_COLOR_PALETTE.orange.base,
  },
  amber: {
    primary: MOTO_COLOR_PALETTE.amber.base,
    secondary: MOTO_COLOR_PALETTE.amber.light,
  },
  purple: {
    primary: MOTO_COLOR_PALETTE.purple.strong,
    secondary: MOTO_COLOR_PALETTE.purple.light,
  },
  greenDeep: {
    primary: MOTO_COLOR_PALETTE.green.strong,
    secondary: MOTO_COLOR_PALETTE.green.light,
  },
  greenVivid: {
    primary: MOTO_COLOR_PALETTE.green.vivid,
    secondary: MOTO_COLOR_PALETTE.green.muted,
  },
  indigo: {
    primary: MOTO_COLOR_PALETTE.indigo.strong,
    secondary: MOTO_COLOR_PALETTE.indigo.light,
  },
  red: {
    primary: MOTO_COLOR_PALETTE.red.strong,
    secondary: "var(--color-white)",
  },
  coral: {
    primary: MOTO_COLOR_PALETTE.coral.strong,
    secondary: MOTO_COLOR_PALETTE.coral.light,
  },
  cyan: {
    primary: MOTO_COLOR_PALETTE.cyan.strong,
    secondary: MOTO_COLOR_PALETTE.cyan.light,
  },
  navy: {
    primary: MOTO_COLOR_PALETTE.navy.strong,
    secondary: MOTO_COLOR_PALETTE.navy.light,
  },
  mint: {
    primary: MOTO_COLOR_PALETTE.mint.strong,
    secondary: MOTO_COLOR_PALETTE.mint.light,
  },
  wine: {
    primary: MOTO_COLOR_PALETTE.wine.strong,
    secondary: MOTO_COLOR_PALETTE.wine.light,
  },
  gold: {
    primary: MOTO_COLOR_PALETTE.gold.strong,
    secondary: MOTO_COLOR_PALETTE.gold.light,
  },
  petrol: {
    primary: MOTO_COLOR_PALETTE.petrol.strong,
    secondary: MOTO_COLOR_PALETTE.petrol.light,
  },
  neutral: {
    primary: MOTO_COLOR_PALETTE.neutral.strong,
    secondary: MOTO_COLOR_PALETTE.neutral.light,
  },
  stone: {
    primary: MOTO_COLOR_PALETTE.stone.strong,
    secondary: MOTO_COLOR_PALETTE.stone.light,
  },
};

// =============================================================================
// Pills
// =============================================================================

export interface PillTone {
  readonly backgroundColor: string;
  readonly textColor: string;
}

/** Hues with a `-soft` tint and a `-strong` label step. */
type RampHue = {
  [H in MotoPaletteHue]: (typeof MOTO_COLOR_PALETTE)[H] extends {
    soft: string;
    strong: string;
  }
    ? H
    : never;
}[MotoPaletteHue];

/** A pill tone: the hue's `-soft` tint with its `-strong` label. */
function softPill(hue: RampHue): PillTone {
  const ramp: { soft: string; strong: string } = MOTO_COLOR_PALETTE[hue];
  return { backgroundColor: ramp.soft, textColor: ramp.strong };
}

/**
 * The one pill tint per hue. `StatusBadge` picks its tones from here, and
 * `getPillTone` finds them by the hue's base hex. A hue missing here (coral,
 * mint) gets the darkened gray-50 pill like any colour outside the palette.
 */
export const PILL_TONES = {
  green: softPill("green"),
  blue: softPill("blue"),
  orange: softPill("orange"),
  red: softPill("red"),
  navy: softPill("navy"),
  cyan: softPill("cyan"),
  petrol: softPill("petrol"),
  stone: softPill("stone"),
  amber: {
    backgroundColor: MOTO_COLOR_PALETTE.amber.pill,
    textColor: MOTO_COLOR_PALETTE.amber.pillText,
  },
  purple: softPill("purple"),
  magenta: softPill("magenta"),
  indigo: softPill("indigo"),
  neutral: {
    backgroundColor: MOTO_COLOR_PALETTE.neutral.soft,
    textColor: MOTO_COLOR_PALETTE.neutral.pillText,
  },
  timeTracking: softPill("timeTracking"),
  wine: softPill("wine"),
  gold: softPill("gold"),
  teal: softPill("teal"),
} as const satisfies Partial<Record<MotoPaletteHue, PillTone>>;

/**
 * Tailwind gray-50 — the pill background every colour outside the palette
 * falls back to, and the surface its label contrast has to be measured
 * against (white would overstate it).
 */
const PILL_FALLBACK_SURFACE = "#F9FAFB";

const PILL_TONES_BY_BASE: ReadonlyMap<string, PillTone> = new Map(
  Object.entries(PILL_TONES).map(([hue, tone]) => [
    MOTO_COLOR_PALETTE[hue as keyof typeof PILL_TONES].base,
    tone,
  ]),
);

/**
 * The pill tone for a hex colour. A palette base hex gets its hue's tone from
 * {@link PILL_TONES}; anything else (a per-room hex, #1324) keeps its own hue
 * as a label darkened to the text minimum on the gray-50 pill, so two custom
 * colours stay two different pills.
 *
 * Accepts a missing color on purpose: an indexed palette lookup is
 * `string | undefined` under `noUncheckedIndexedAccess`, and a status the
 * backend adds before the frontend knows it must not blow up a badge. Falls
 * back to the neutral label.
 */
export function getPillTone(color: string | null | undefined): PillTone {
  if (color === null || color === undefined || color.trim() === "") {
    return {
      backgroundColor: PILL_FALLBACK_SURFACE,
      textColor: NEUTRAL_TEXT_COLOR,
    };
  }

  return (
    PILL_TONES_BY_BASE.get(color.toUpperCase()) ?? {
      backgroundColor: PILL_FALLBACK_SURFACE,
      textColor: getAccessibleTextColor(color, PILL_FALLBACK_SURFACE),
    }
  );
}

// =============================================================================
// Tinted surfaces
// =============================================================================

/**
 * The tinted `-soft` surface of a hue as utility classes: one table for Alert,
 * InsetPanel, IconTile and VisibilityBadge, so the same meaning gets the same
 * tint everywhere (#6564). Written out in full so Tailwind finds the classes.
 */
export const TINT_CLASSES = {
  red: {
    surface: "bg-moto-red-soft",
    text: "text-moto-red-strong",
    border: "border-moto-red/20",
  },
  green: {
    surface: "bg-moto-green-soft",
    text: "text-moto-green-strong",
    border: "border-moto-green/20",
  },
  orange: {
    surface: "bg-moto-orange-soft",
    text: "text-moto-orange-strong",
    border: "border-moto-orange/20",
  },
  blue: {
    surface: "bg-moto-blue-soft",
    text: "text-moto-blue-strong",
    border: "border-moto-blue/20",
  },
} as const satisfies Partial<
  Record<MotoPaletteHue, { surface: string; text: string; border: string }>
>;

// =============================================================================
// Text-safe shades
// =============================================================================

/**
 * Text shades of palette hues, dark enough to carry a label. The raw base
 * hexes are fills: on white #83CD2D reaches 2.0:1, #F78C10 2.4:1 and #EAB308
 * 1.9:1, all far under the WCAG minimum for normal text. These keep the hue and
 * only take brightness away.
 *
 * Calibrated against Tailwind gray-50 (#F9FAFB), the darker of the two surfaces
 * they run on (the badge pill; cards are white), so every entry holds on both.
 * The comments carry the ratio on gray-50; on white each value is a little
 * better still.
 *
 * A hue missing here starts from its base hex and is darkened arithmetically
 * by `getAccessibleTextColor`, as is every colour outside the palette (per-room
 * hexes), against the surface it is actually drawn on.
 *
 * These are NOT the pill label colours: those are darkened further to sit on a
 * tinted pill, which reads muddy on a large figure on white (#8A5600 turns
 * brown). Both tables exist on purpose.
 */
const TEXT_SAFE_SHADES: Partial<Record<MotoPaletteHue, string>> = {
  green: MOTO_COLOR_PALETTE.green.strong, // 6.0:1
  blue: "#3d6ab8", // 5.1:1
  orange: "#a35c0b", // 4.9:1
  magenta: "#af39c1", // 4.8:1
  amber: "#8b6a05", // 4.8:1
  neutral: MOTO_COLOR_PALETTE.neutral.strong, // 9.9:1 — neutral figures read as gray-700, not gray-500
  // Purple #7C3AED is absent on purpose: at 5.5:1 the raw purple already
  // clears the target and is returned unchanged.
};

const TEXT_SAFE_SHADES_BY_BASE: ReadonlyMap<string, string> = new Map(
  Object.entries(TEXT_SAFE_SHADES).map(([hue, shade]) => [
    MOTO_COLOR_PALETTE[hue as MotoPaletteHue].base.toLowerCase(),
    shade,
  ]),
);

const SURFACE_WHITE = "#FFFFFF";

/**
 * WCAG AA for normal text is 4.5:1; the target sits above it so a shade never
 * lands on the boundary, where a rounded channel decides pass or fail. Normal
 * text is the strict case, so one target covers large text too.
 */
const TEXT_CONTRAST_TARGET = 4.75;

/** Used when there is no usable color to darken. */
const NEUTRAL_TEXT_COLOR = MOTO_COLOR_PALETTE.neutral.strong;

const accessibleTextCache = new Map<string, string>();

/**
 * Returns a text color for `color` that clears {@link TEXT_CONTRAST_TARGET}
 * against `background` — pass the surface the text actually sits on, since the
 * same shade passes on white and fails on gray-50. Palette hues start from
 * {@link TEXT_SAFE_SHADES}, anything else (a per-room hex, a status color
 * outside the palette) from the color itself; either is scaled down
 * channel-wise until the ratio holds, which keeps the hue. Colors that already
 * pass are returned unchanged.
 *
 * Use it for the LABEL only. Dots, bars, and other fills keep the raw color —
 * they carry no text and contrast rules do not apply the same way.
 *
 * Accepts a missing color: an indexed palette lookup is `string | undefined`
 * under `noUncheckedIndexedAccess`, and a status the backend adds before the
 * frontend knows it must not blow up a badge. It falls back to neutral gray.
 */
export function getAccessibleTextColor(
  color: string | null | undefined,
  background: string = SURFACE_WHITE,
): string {
  if (color === null || color === undefined || color.trim() === "") {
    return NEUTRAL_TEXT_COLOR;
  }

  const key = `${color.trim().toLowerCase()}|${background.trim().toLowerCase()}`;

  const cached = accessibleTextCache.get(key);
  if (cached !== undefined) {
    return cached;
  }

  const resolved = resolveAccessibleTextColor(color, background);
  accessibleTextCache.set(key, resolved);
  return resolved;
}

function resolveAccessibleTextColor(color: string, background: string): string {
  const candidate =
    TEXT_SAFE_SHADES_BY_BASE.get(color.trim().toLowerCase()) ?? color;

  const rgb = hexToRgb(candidate);
  const backgroundRgb = hexToRgb(background);
  if (!rgb || !backgroundRgb) {
    // Non-hex input (a CSS keyword, a var()): darkening it is not possible, so
    // fall back to the neutral text color rather than passing it through.
    return NEUTRAL_TEXT_COLOR;
  }

  if (contrastRatio(rgb, backgroundRgb) >= TEXT_CONTRAST_TARGET) {
    return candidate;
  }

  return rgbToHex(darkenToContrast(rgb, backgroundRgb, TEXT_CONTRAST_TARGET));
}

/**
 * Largest brightness factor that still clears `target`, found by bisection.
 * The rounding to integer channels happens inside the predicate — rounding
 * afterwards can push a borderline result back below the target.
 */
function darkenToContrast(
  rgb: RgbColor,
  background: RgbColor,
  target: number,
): RgbColor {
  let tooDark = 0;
  let tooBright = 1;

  for (let i = 0; i < 24; i++) {
    const factor = (tooDark + tooBright) / 2;
    if (contrastRatio(scaleChannels(rgb, factor), background) >= target) {
      tooDark = factor;
    } else {
      tooBright = factor;
    }
  }

  return scaleChannels(rgb, tooDark);
}

function scaleChannels(rgb: RgbColor, factor: number): RgbColor {
  return {
    r: Math.round(rgb.r * factor),
    g: Math.round(rgb.g * factor),
    b: Math.round(rgb.b * factor),
  };
}

/** WCAG relative luminance (sRGB). */
function relativeLuminance({ r, g, b }: RgbColor): number {
  const channel = (value: number): number => {
    const srgb = value / 255;
    return srgb <= 0.03928
      ? srgb / 12.92
      : Math.pow((srgb + 0.055) / 1.055, 2.4);
  };

  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/** WCAG contrast ratio between two colors. */
function contrastRatio(a: RgbColor, b: RgbColor): number {
  const lumA = relativeLuminance(a);
  const lumB = relativeLuminance(b);
  return (Math.max(lumA, lumB) + 0.05) / (Math.min(lumA, lumB) + 0.05);
}

function rgbToHex({ r, g, b }: RgbColor): string {
  return `#${[r, g, b].map((v) => v.toString(16).padStart(2, "0")).join("")}`;
}

interface RgbColor {
  r: number;
  g: number;
  b: number;
}

function hexToRgb(hexColor: string): RgbColor | null {
  const sanitized = hexColor.trim().replace("#", "");
  if (sanitized.length !== 6) {
    return null;
  }

  const r = Number.parseInt(sanitized.slice(0, 2), 16);
  const g = Number.parseInt(sanitized.slice(2, 4), 16);
  const b = Number.parseInt(sanitized.slice(4, 6), 16);

  if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) {
    return null;
  }

  return { r, g, b };
}
