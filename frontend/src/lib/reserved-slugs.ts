/**
 * Slugs and subdomains reserved for infrastructure — never valid as tenant identifiers.
 *
 * This is the single source of truth for the frontend. Used by:
 * - middleware.ts (blocks reserved subdomains at request level)
 * - [tenant]/layout.tsx (blocks reserved slugs at route level)
 *
 * Backend equivalent: backend/models/platform/organization.go (reservedSlugs map).
 * Both lists MUST stay in sync. If you add a slug here, add it in the backend too.
 */
export const RESERVED_SLUGS = new Set(["www", "api", "operator"]);
