import { MOTO_COLOR_PALETTE } from "~/components/ui/kit-palette";

/**
 * The colours of the planning surfaces (#6573): one entry per meaning, so a
 * coverage gap, an understaffed figure or a cancelled block looks the same in
 * every planning piece. Values come from the kit palette.
 */
export const PLANNING_COLORS = {
  /** Understaffing red. Only ever a text colour, never a fill. */
  understaffedText: MOTO_COLOR_PALETTE.red.base,
  /** A slot nobody covers: the coverage dot and the day chip's counter dot. */
  gap: MOTO_COLOR_PALETTE.orange.base,
  /** Coverage dot of a covered slot. */
  covered: MOTO_COLOR_PALETTE.neutral.light,
  /** Coverage dot of a gap someone accepted ("bewusst unbesetzt"). */
  acknowledged: MOTO_COLOR_PALETTE.neutral.base,
  /** Edge of a cancelled block and its legend glyph. */
  cancelled: MOTO_COLOR_PALETTE.neutral.light,
  /** Hatch strokes of the absence legend glyph. */
  hatchStroke: MOTO_COLOR_PALETTE.neutral.base,
} as const;

/**
 * Delta tint of a free-form coverage label, text colour only, never a fill.
 * Both steps clear 4.5:1 on white; amber.base (1.9:1) is never a text colour
 * (#5576, Entscheidung 3), so Überdeckung takes amber.strong (7.1:1).
 */
export const COVERAGE_DELTA_TEXT_COLOR = {
  under: PLANNING_COLORS.understaffedText,
  over: MOTO_COLOR_PALETTE.amber.strong,
} as const;
