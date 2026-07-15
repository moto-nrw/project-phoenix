// Brand link component for header
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import Link from "next/link";
import Image from "next/image";

/**
 * Brand link with logo and text
 */
interface BrandLinkProps {
  readonly isScrolled?: boolean;
  readonly href?: string;
  readonly label?: string | null;
}

export function BrandLink({
  isScrolled = false,
  href = "/dashboard",
  label,
}: BrandLinkProps) {
  const displayLabel = label?.trim() || "moto";
  const usesTenantLabel = Boolean(label?.trim());

  return (
    <Link
      href={href}
      className="group flex max-w-[180px] min-w-0 items-center space-x-3 sm:max-w-[240px] lg:max-w-[280px]"
    >
      <div>
        <Image
          src="/images/moto_transparent.webp"
          alt=""
          width={56}
          height={40}
          className="h-8 w-11 object-contain sm:h-10 sm:w-14"
          priority
        />
      </div>

      <div className="flex min-w-0 items-center space-x-3">
        <span
          className={`truncate leading-none transition-all duration-150 ${
            usesTenantLabel
              ? `font-semibold text-gray-900 ${
                  isScrolled ? "text-sm lg:text-base" : "text-base lg:text-lg"
                }`
              : `[font-family:var(--font-moto)] font-bold text-gray-950 ${
                  isScrolled ? "text-xl lg:text-[22px]" : "text-[22px]"
                }`
          }`}
        >
          {displayLabel}
        </span>
      </div>
    </Link>
  );
}

/**
 * Vertical separator for breadcrumb area
 */
export function BreadcrumbDivider() {
  return <div className="hidden h-5 w-px bg-gray-300 md:block" />;
}
