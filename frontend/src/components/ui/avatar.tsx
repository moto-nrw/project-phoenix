// Shared circular avatar with image + initials fallback.
//
// Single source of truth for every avatar in the app — extracted from the
// previous private UserAvatar in components/dashboard/header/profile-dropdown
// (which now consumes this) so user, staff, and student photos all render
// identically. Pass `imageUrl` when you have one, `name` always; the
// component falls back to deterministic initials so a missing image never
// produces a broken layout.

"use client";

import Image from "next/image";
import { useEffect, useState } from "react";
import { getInitials } from "~/lib/format-utils";

export type AvatarSize = "xs" | "sm" | "md" | "lg" | "xl";

interface AvatarProps {
  /** Public image URL (e.g. "/uploads/student-photos/..."). Falsy → initials. */
  readonly imageUrl?: string | null;
  /** Used for accessible alt text and to compute the initials fallback. */
  readonly name: string;
  /** Visual size. Sizes match the original UserAvatar so existing places stay pixel-identical. */
  readonly size?: AvatarSize;
  /** Optional override class for spacing concerns at the call site. */
  readonly className?: string;
}

// Size table is the only place pixel dimensions live. Adding a size? Add it
// here, not at every call site. The Tailwind sizes are deliberately arbitrary
// values so they survive a future Tailwind version bump without regressions.
const SIZE_CLASSES: Record<AvatarSize, string> = {
  xs: "w-6 h-6 text-[10px]",
  sm: "w-8 h-8 text-sm",
  md: "w-11 h-11 text-base",
  lg: "w-16 h-16 text-xl",
  xl: "w-24 h-24 text-3xl",
};

// `sizes` attribute for the next/image renderer, matched to SIZE_CLASSES so
// the browser hints land in the right ballpark. next/image is already in
// `unoptimized` mode (we serve the same JPEG everywhere) so this is purely
// a srcset/density-decision hint.
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

  // Track image-load failures so a stale or revoked URL falls back to the
  // initials gradient instead of rendering the browser's broken-image
  // glyph. Hit cases:
  //   - another tab deletes the photo → cards still hold the old
  //     /api/students/{id}/photo/{filename} URL until SWR revalidates;
  //     the proxy 404s in the meantime.
  //   - operator disables operations.student_photos_enabled → the serve
  //     route 403s for cached URLs until TenantContext re-resolves.
  //   - photo file unlinked but DB row still points at it (purge race
  //     window before the new atomic CTE; defense in depth here).
  // Reset whenever imageUrl changes so a fresh upload of the same student
  // doesn't stay in the failed-fallback state from a prior URL.
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    setFailed(false);
  }, [imageUrl]);

  const showImage = Boolean(imageUrl) && !failed;

  return (
    <div
      className={`relative ${sizeClass} ${ringClass} flex flex-shrink-0 items-center justify-center overflow-hidden rounded-full font-semibold text-white ${className ?? ""}`}
      style={{
        // Brand gradient is the initials background, identical to the
        // original profile-dropdown UserAvatar so user-avatar fallbacks
        // stay pixel-identical after this extraction.
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
          // `unoptimized` keeps next/image from running its own pipeline:
          // student photos are already compressed client-side via
          // compressAvatar() to ~300 KB / 512² JPEG, and avatars come from
          // an authenticated route, so the optimisation server can't
          // re-fetch them anyway.
          unoptimized
          onError={() => setFailed(true)}
        />
      ) : (
        initials
      )}
    </div>
  );
}
