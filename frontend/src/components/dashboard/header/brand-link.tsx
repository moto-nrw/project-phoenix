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
      className="h-8 w-11 shrink-0 object-contain"
      priority
    />
  );
}

/**
 * Typography for the brand label. Uses leading-tight (not leading-none):
 * the span truncates via overflow-hidden, and a line box smaller than the
 * font's ascent+descent clips descenders (g, j, p, q, y).
 *
 * Eine feste Größe für die 48px hohe Kopfzeile (#2827): der frühere
 * Scroll-Zustand ist der Normalzustand, es gibt nichts mehr zu schrumpfen.
 */
export function brandLabelClass(usesTenantLabel: boolean): string {
  return `truncate leading-tight ${
    usesTenantLabel
      ? "text-base font-semibold text-gray-900"
      : "[font-family:var(--font-moto)] text-xl font-bold text-gray-950"
  }`;
}

/**
 * Zweite Zeile unter dem Schulnamen: der Träger der Schule (#4656). Nur
 * gesetzt, wenn der Träger mehr als eine Schule hat; sonst bleibt die
 * Kopfzeile, wie sie ist.
 */
export function BrandTraegerLine({
  traegerName,
}: Readonly<{ traegerName?: string | null }>) {
  if (!traegerName) return null;
  return (
    <span className="truncate text-xs leading-tight text-gray-500">
      Träger: {traegerName}
    </span>
  );
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
  readonly href?: string;
  readonly label?: string | null;
  /**
   * Unterhalb dieser Breite nur das Logo zeigen, damit die Kopfzeile Platz
   * für die Ortsangabe der Seite hat: `"lg"` im Elternportal, `"md"` im
   * Mitarbeiterportal (dort beginnen die Brotkrumen ab md).
   */
  readonly hideLabelBelow?: "md" | "lg";
  /** Träger der Schule als zweite Zeile, siehe `BrandTraegerLine`. */
  readonly traegerName?: string | null;
}

export function BrandLink({
  href = "/home",
  label,
  hideLabelBelow,
  traegerName,
}: BrandLinkProps) {
  const displayLabel = label?.trim() || "moto";
  const usesTenantLabel = Boolean(label?.trim());

  return (
    <NavLink
      href={href}
      className="group flex max-w-[180px] min-w-0 items-center gap-2.5 sm:max-w-[240px] lg:max-w-[280px]"
    >
      <BrandLogo />

      <div
        className={`min-w-0 flex-col justify-center ${
          hideLabelBelow ? HIDE_LABEL_CLASS[hideLabelBelow] : "flex"
        }`}
      >
        <span className={brandLabelClass(usesTenantLabel)}>{displayLabel}</span>
        <BrandTraegerLine traegerName={traegerName} />
      </div>
    </NavLink>
  );
}

/**
 * Vertical separator for breadcrumb area
 */
export function BreadcrumbDivider() {
  return <div className="hidden h-4 w-px bg-gray-300 md:block" />;
}
