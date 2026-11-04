import {
  MOTO_COLOR_PALETTE,
  type MotoDuotoneTone,
} from "~/lib/location-helper";

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
