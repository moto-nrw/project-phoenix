"use client";

import { loginImageSrc } from "~/lib/tenant-api";
import { useTenantSafe } from "~/lib/tenant-context";
import type { QrBrandLogo } from "./qr-code-dialog";

/**
 * The picture a school stored for its login page (Einstellungen,
 * Personalisierung), as the logo of its QR codes. `undefined` without one:
 * the code then carries the moto logo.
 */
export function useSchoolQrLogo(): QrBrandLogo | undefined {
  const stored = useTenantSafe()?.tenant?.settings?.loginImageUrl;
  if (!stored) return undefined;
  return { src: loginImageSrc(stored), label: "Schule" };
}

/**
 * The logo of the Träger (Einstellungen, Briefkopf). The dialog asks for the
 * picture itself and falls back to the moto logo when none is stored, so no
 * page has to read the master data for it.
 */
export const TRAEGER_QR_LOGO: QrBrandLogo = {
  src: "/api/traeger/master-data/logo",
  label: "Träger",
};
