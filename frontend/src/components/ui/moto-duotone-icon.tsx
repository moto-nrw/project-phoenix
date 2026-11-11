import type { Icon as PhosphorIcon, IconProps } from "@phosphor-icons/react";
import type { CSSProperties } from "react";
import type { MotoDuotoneTone } from "~/lib/location-helper";
import { MOTO_DUOTONE_TONES } from "~/components/ui/moto-duotone-tones";
import { cn } from "~/lib/utils";

export interface MotoDuotoneIconProps {
  readonly icon: PhosphorIcon;
  readonly tone: MotoDuotoneTone;
  /**
   * `tone` applies the configured pair, `inherit` uses the surrounding
   * foreground color for both layers, and `class` leaves both colors to CSS.
   */
  readonly colorMode?: "tone" | "inherit" | "class";
  readonly size?: IconProps["size"];
  readonly weight?: IconProps["weight"];
  readonly className?: string;
  readonly label?: string;
}

export function MotoDuotoneIcon({
  icon: Icon,
  tone,
  colorMode = "tone",
  size = 32,
  weight = "duotone",
  className,
  label,
}: MotoDuotoneIconProps) {
  const colors = MOTO_DUOTONE_TONES[tone];
  const iconStyle =
    colorMode === "inherit"
      ? ({ "--moto-icon-secondary": "currentColor" } satisfies CSSProperties & {
          "--moto-icon-secondary": string;
        })
      : colorMode === "class"
        ? undefined
        : ({
            color: colors.primary,
            "--moto-icon-secondary": colors.secondary,
          } satisfies CSSProperties & { "--moto-icon-secondary": string });

  return (
    <Icon
      size={size}
      weight={weight}
      className={cn("moto-duotone-icon shrink-0", className)}
      data-moto-duotone-tone={tone}
      style={iconStyle}
      aria-hidden={label ? undefined : true}
      aria-label={label}
      role={label ? "img" : undefined}
    />
  );
}
