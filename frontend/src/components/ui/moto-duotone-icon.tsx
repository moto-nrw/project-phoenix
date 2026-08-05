import type { Icon as PhosphorIcon, IconProps } from "@phosphor-icons/react";
import type { CSSProperties } from "react";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { cn } from "~/lib/utils";

export type MotoDuotoneTone =
  | "blue"
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

export const MOTO_DUOTONE_TONES: Record<
  MotoDuotoneTone,
  { primary: string; secondary: string }
> = {
  blue: {
    primary: MOTO_COLOR_PALETTE.blue.strong,
    secondary: MOTO_COLOR_PALETTE.blue.light,
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

export interface MotoDuotoneIconProps {
  readonly icon: PhosphorIcon;
  readonly tone: MotoDuotoneTone;
  readonly size?: IconProps["size"];
  readonly className?: string;
  readonly label?: string;
}

export function MotoDuotoneIcon({
  icon: Icon,
  tone,
  size = 32,
  className,
  label,
}: MotoDuotoneIconProps) {
  const colors = MOTO_DUOTONE_TONES[tone];

  return (
    <Icon
      size={size}
      weight="duotone"
      className={cn("moto-duotone-icon shrink-0", className)}
      data-moto-duotone-tone={tone}
      style={
        {
          color: colors.primary,
          "--moto-icon-secondary": colors.secondary,
        } as CSSProperties
      }
      aria-hidden={label ? undefined : true}
      aria-label={label}
      role={label ? "img" : undefined}
    />
  );
}
