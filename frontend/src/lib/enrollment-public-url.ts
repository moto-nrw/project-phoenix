"use client";

import { useEffect, useMemo, useState } from "react";

interface EnrollmentPublicUrlParams {
  readonly tenantSlug: string | null | undefined;
  readonly phaseId?: string | null;
}

interface BrowserLocationSnapshot {
  readonly origin: string;
  readonly hostname: string;
}

/**
 * Builds an absolute public enrollment URL for the current tenant.
 *
 * In subdomain mode the public route is /enroll[/phaseId]. In path mode the
 * tenant slug must stay in the URL: /{tenantSlug}/enroll[/phaseId].
 */
export function buildEnrollmentPublicUrl({
  tenantSlug,
  phaseId,
}: EnrollmentPublicUrlParams): string;
export function buildEnrollmentPublicUrl(
  { tenantSlug, phaseId }: EnrollmentPublicUrlParams,
  location: BrowserLocationSnapshot,
): string;
export function buildEnrollmentPublicUrl(
  { tenantSlug, phaseId }: EnrollmentPublicUrlParams,
  location?: BrowserLocationSnapshot,
): string {
  const loc =
    location ??
    (typeof window !== "undefined"
      ? {
          origin: window.location.origin,
          hostname: window.location.hostname,
        }
      : null);
  if (!loc || !tenantSlug) return "";

  const path = phaseId
    ? `/anmeldung/${encodeURIComponent(phaseId)}`
    : "/anmeldung";
  const inSubdomainMode = loc.hostname.startsWith(`${tenantSlug}.`);

  if (inSubdomainMode) {
    return `${loc.origin}${path}`;
  }
  return `${loc.origin}/${encodeURIComponent(tenantSlug)}${path}`;
}

export function useEnrollmentPublicUrl({
  tenantSlug,
  phaseId,
}: EnrollmentPublicUrlParams) {
  const [location, setLocation] = useState<BrowserLocationSnapshot | null>(
    null,
  );

  useEffect(() => {
    setLocation({
      origin: window.location.origin,
      hostname: window.location.hostname,
    });
  }, []);

  return useMemo(() => {
    if (!location) return "";
    return buildEnrollmentPublicUrl({ tenantSlug, phaseId }, location);
  }, [location, tenantSlug, phaseId]);
}
