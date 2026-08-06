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
  readonly className?: string;
}

// Soft brand-green fallback for the initials state paired with the stronger
// green letter color, matching the child cards in the parent portal. The
// background is the OPAQUE equivalent of LOCATION_COLORS.GROUP_ROOM (#83CD2D)
// at 15% over white — kept opaque so a dotted/colored page background never
// shows through the avatar.
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

  return (
    <div
      className={cn(
        "relative flex flex-shrink-0 items-center justify-center overflow-hidden font-semibold",
        sizeClass,
        shape === "circle" ? "rounded-full" : "rounded-xl",
        className,
      )}
      style={{
        background: showImage ? "transparent" : FALLBACK_BG,
        color: showImage ? undefined : FALLBACK_TEXT,
      }}
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
