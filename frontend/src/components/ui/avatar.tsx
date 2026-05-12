// Shared circular avatar with image + initials fallback.

"use client";

import Image from "next/image";
import { useEffect, useState } from "react";
import { getInitials } from "~/lib/format-utils";

type AvatarSize = "xs" | "sm" | "md" | "lg" | "xl";

interface AvatarProps {
  readonly imageUrl?: string | null;
  readonly name: string;
  readonly size?: AvatarSize;
  readonly className?: string;
}

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
  className,
}: AvatarProps) {
  const initials = getInitials(name);
  const sizeClass = SIZE_CLASSES[size];
  const ringClass =
    size === "xs" || size === "sm"
      ? "shadow-sm ring-2 ring-white"
      : "shadow-md";

  // Stale URL → initials fallback instead of broken-image glyph. Reset on
  // imageUrl change so a fresh upload doesn't stay stuck in fallback state.
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setFailed(false);
  }, [imageUrl]);

  const showImage = Boolean(imageUrl) && !failed;

  return (
    <div
      className={`relative ${sizeClass} ${ringClass} flex flex-shrink-0 items-center justify-center overflow-hidden rounded-full font-semibold text-white ${className ?? ""}`}
      style={{
        background: showImage
          ? "transparent"
          : "linear-gradient(135deg, #5080d8, #83cd2d)",
      }}
      aria-label={name}
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
