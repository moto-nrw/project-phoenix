export function tenantAwarePath(
  path: string,
  tenantSlug: string | null | undefined,
): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  if (!tenantSlug) return normalizedPath;

  const inSubdomainMode =
    typeof window !== "undefined" &&
    window.location.hostname.startsWith(`${tenantSlug}.`);
  if (inSubdomainMode) return normalizedPath;

  return `/${tenantSlug}${normalizedPath}`;
}
