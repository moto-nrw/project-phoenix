// Tenant resolution uses the school's subdomain, which is a DNS label.
// Keep this in sync with backend/modules/organizationtenancy/organizationtenancy.go
// slugPattern and MaxDNSLabelLength.
const TENANT_SLUG_PATTERN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;
const MAX_TENANT_SLUG_LENGTH = 63;

export function isValidTenantSlug(slug: string): boolean {
  return (
    slug.length <= MAX_TENANT_SLUG_LENGTH && TENANT_SLUG_PATTERN.test(slug)
  );
}
