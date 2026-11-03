import type { Icon as PhosphorIcon, IconProps } from "@phosphor-icons/react";
import type { CSSProperties } from "react";
import type { MotoDuotoneTone } from "~/lib/location-helper";
import { MOTO_DUOTONE_TONES } from "~/components/ui/moto-duotone-tones";
import { cn } from "~/lib/utils";
export { MOTO_DUOTONE_TONES } from "~/components/ui/moto-duotone-tones";

export interface MotoDuotoneIconProps {
  readonly icon: PhosphorIcon;
  readonly tone: MotoDuotoneTone;
  /** Use the surrounding foreground color for both icon layers. */
  readonly colorMode?: "tone" | "inherit";
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
  const iconStyle: CSSProperties & { "--moto-icon-secondary": string } =
    colorMode === "inherit"
      ? { "--moto-icon-secondary": "currentColor" }
      : {
          color: colors.primary,
          "--moto-icon-secondary": colors.secondary,
        };

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
