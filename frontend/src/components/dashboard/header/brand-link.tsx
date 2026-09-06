// Brand link component for header
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import { NavLink } from "~/components/ui/nav-link";
import Image from "next/image";

/**
 * Logo image shared by BrandLink and BrandTenantSwitcher.
 */
export function BrandLogo() {
  return (
    <Image
      src="/images/moto_transparent.webp"
      alt=""
      width={56}
      height={40}
      className="h-8 w-11 shrink-0 object-contain sm:h-10 sm:w-14"
      priority
    />
  );
}

/**
 * Typography for the brand label. Uses leading-tight (not leading-none):
 * the span truncates via overflow-hidden, and a line box smaller than the
 * font's ascent+descent clips descenders (g, j, p, q, y).
 */
export function brandLabelClass(
  isScrolled: boolean,
  usesTenantLabel: boolean,
): string {
  return `truncate leading-tight transition-all duration-150 ${
    usesTenantLabel
      ? `font-semibold text-gray-900 ${
          isScrolled ? "text-sm lg:text-base" : "text-base lg:text-lg"
        }`
      : `[font-family:var(--font-moto)] font-bold text-gray-950 ${
          isScrolled ? "text-xl lg:text-[22px]" : "text-[22px]"
        }`
  }`;
}

/**
 * Tailwind kennt keine zusammengesetzten Klassennamen zur Laufzeit, deshalb
 * beide Varianten als vollständige Literale.
 */
const HIDE_LABEL_CLASS = {
  md: "hidden md:flex",
  lg: "hidden lg:flex",
} as const;

/**
 * Brand link with logo and text
 */
interface BrandLinkProps {
  readonly isScrolled?: boolean;
  readonly href?: string;
  readonly label?: string | null;
  /**
   * Unterhalb dieser Breite nur das Logo zeigen, damit die Kopfzeile Platz
   * für die Ortsangabe der Seite hat: `"lg"` im Elternportal, `"md"` im
   * Mitarbeiterportal (dort beginnen die Brotkrumen ab md).
   */
  readonly hideLabelBelow?: "md" | "lg";
}

export function BrandLink({
  isScrolled = false,
  href = "/dashboard",
  label,
  hideLabelBelow,
}: BrandLinkProps) {
  const displayLabel = label?.trim() || "moto";
  const usesTenantLabel = Boolean(label?.trim());

  return (
    <NavLink
      href={href}
      className="group flex max-w-[180px] min-w-0 items-center space-x-3 sm:max-w-[240px] lg:max-w-[280px]"
    >
      <BrandLogo />

      <div
        className={`min-w-0 items-center space-x-3 ${
          hideLabelBelow ? HIDE_LABEL_CLASS[hideLabelBelow] : "flex"
        }`}
      >
        <span className={brandLabelClass(isScrolled, usesTenantLabel)}>
          {displayLabel}
        </span>
      </div>
    </NavLink>
  );
}

/**
 * Vertical separator for breadcrumb area
 */
export function BreadcrumbDivider() {
  return <div className="hidden h-5 w-px bg-gray-300 md:block" />;
}
