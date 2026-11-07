// Shared circular avatar with image + initials fallback.

"use client";

import Image from "next/image";
import { useEffect, useState } from "react";
import { getInitials } from "~/lib/format-utils";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { cn } from "~/lib/utils";

type AvatarSize = "xs" | "sm" | "md" | "lg" | "xl";

interface AvatarProps {
  readonly imageUrl?: string | null;
  readonly name: string;
  readonly size?: AvatarSize;
  readonly shape?: "circle" | "rounded";
  /** Use when the avatar repeats adjacent visible text. */
  readonly decorative?: boolean;
  /** Fläche der Initialen: `green` für Kinder, `neutral` für Erwachsene. */
  readonly tone?: "green" | "neutral";
  readonly className?: string;
}

// Soft brand-green initials (child cards of the parent portal): the OPAQUE
// equivalent of LOCATION_COLORS.GROUP_ROOM (#83CD2D) at 15% over white, so a
// dotted/colored page background never shows through the avatar.
const FALLBACK_BG = MOTO_COLOR_PALETTE.green.soft;
const FALLBACK_TEXT = MOTO_COLOR_PALETTE.green.strong;

const SIZE_CLASSES: Record<AvatarSize, string> = {
  xs: "w-6 h-6 text-[10px]",
  sm: "w-8 h-8 text-sm",
  md: "w-11 h-11 text-base",
  lg: "w-16 h-16 text-xl",
  xl: "w-24 h-24 text-3xl",
};

const IMAGE_SIZES_ATTR: Record<AvatarSize, string> = {
  xs: "24px",
  sm: "32px",
  md: "44px",
  lg: "64px",
  xl: "96px",
};

export function Avatar({
  imageUrl,
  name,
  size = "sm",
  shape = "circle",
  decorative = false,
  tone = "green",
  className,
}: AvatarProps) {
  const initials = getInitials(name);
  const sizeClass = SIZE_CLASSES[size];

  // Stale URL → initials fallback instead of broken-image glyph. Reset on
  // imageUrl change so a fresh upload doesn't stay stuck in fallback state.
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setFailed(false);
  }, [imageUrl]);

  const showImage = Boolean(imageUrl) && !failed;
  const fallbackStyle =
    tone === "neutral"
      ? { background: "var(--color-gray-100)", color: "var(--color-gray-700)" }
      : { background: FALLBACK_BG, color: FALLBACK_TEXT };

  return (
    <div
      className={cn(
        "relative flex flex-shrink-0 items-center justify-center overflow-hidden font-semibold",
        sizeClass,
        shape === "circle" ? "rounded-full" : "rounded-xl",
        className,
      )}
      style={showImage ? { background: "transparent" } : fallbackStyle}
      aria-hidden={decorative || undefined}
      aria-label={decorative ? undefined : name}
    >
      {showImage && imageUrl ? (
        <Image
          src={imageUrl}
          alt={name}
          fill
          className="object-cover"
          sizes={IMAGE_SIZES_ATTR[size]}
          // Photos are pre-compressed and live behind authenticated routes —
          // next/image's optimisation server can't re-fetch them.
          unoptimized
          onError={() => setFailed(true)}
        />
      ) : (
        initials
      )}
    </div>
  );
}
