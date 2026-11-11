"use client";

import { loginImageSrc } from "~/lib/tenant-api";
import { useTenantSafe } from "~/lib/tenant-context";

/** The logo a school or Träger has stored in moto. */
export interface QrBrandLogo {
  /** Same-origin address of the picture. */
  readonly src: string;
  /** Who it belongs to, as the choice names it. */
  readonly label: string;
}

/** Whose stored logo a QR code carries by default. */
export type QrBrand = "school" | "traeger";

/**
 * The stored logo for a QR code, `undefined` without one: the code then
 * carries the moto logo.
 *
 * - `school`: the picture of the school's login page (Einstellungen,
 *   Personalisierung).
 * - `traeger`: the logo of the Träger (Einstellungen, Briefkopf). The caller
 *   asks for the picture itself and falls back when none is stored, so no
 *   page has to read the master data for it.
 *
 * Resolved inside the QR components, which load on demand: a page that shows
 * a link does not need the tenant context for it.
 */
export function useQrBrandLogo(brand: QrBrand): QrBrandLogo | undefined {
  const stored = useTenantSafe()?.tenant?.settings?.loginImageUrl;
  if (brand === "traeger") {
    return { src: "/api/traeger/master-data/logo", label: "Träger" };
  }
  return stored ? { src: loginImageSrc(stored), label: "Schule" } : undefined;
}
