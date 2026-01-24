/**
 * Validation utilities for organization slugs.
 * Mirrors the validation logic in betterauth/src/validation.ts
 */

/**
 * Reserved slugs that cannot be used for organization subdomains.
 * These are blocked to prevent conflicts with system routes and services.
 */
export const RESERVED_SLUGS = [
  // System routes
  "api",
  "auth",
  "admin",
  "app",
  "www",
  // Infrastructure
  "staging",
  "demo",
  "test",
  "dev",
  "prod",
  "production",
  // Services
  "mail",
  "smtp",
  "ftp",
  "cdn",
  "static",
  "assets",
  "images",
  "files",
  // Common patterns
  "login",
  "signup",
  "register",
  "dashboard",
  "settings",
  "account",
  "profile",
  "help",
  "support",
  "docs",
  "status",
] as const;

export type ReservedSlug = (typeof RESERVED_SLUGS)[number];

/**
 * Slug validation result
 */
export interface SlugValidationResult {
  valid: boolean;
  error?: string;
}

/**
 * Validates an organization slug for subdomain usage.
 *
 * Rules:
 * - 3-30 characters
 * - Lowercase alphanumeric and hyphens only
 * - Cannot start or end with a hyphen
 * - Cannot have consecutive hyphens
 * - Cannot be a reserved slug
 *
 * @param slug - The slug to validate
 * @returns Validation result with error message if invalid
 */
export function validateSlug(slug: string): SlugValidationResult {
  // Check if empty
  if (!slug) {
    return { valid: false, error: "Subdomain ist erforderlich" };
  }

  // Convert to lowercase for validation
  const normalized = slug.toLowerCase();

  // Check length
  if (normalized.length < 3) {
    return {
      valid: false,
      error: "Subdomain muss mindestens 3 Zeichen haben",
    };
  }

  if (normalized.length > 30) {
    return {
      valid: false,
      error: "Subdomain darf maximal 30 Zeichen haben",
    };
  }

  // Check for valid characters (lowercase alphanumeric and hyphens)
  if (!/^[a-z0-9-]+$/.test(normalized)) {
    return {
      valid: false,
      error:
        "Subdomain darf nur Kleinbuchstaben, Zahlen und Bindestriche enthalten",
    };
  }

  // Check if starts with hyphen
  if (normalized.startsWith("-")) {
    return {
      valid: false,
      error: "Subdomain darf nicht mit einem Bindestrich beginnen",
    };
  }

  // Check if ends with hyphen
  if (normalized.endsWith("-")) {
    return {
      valid: false,
      error: "Subdomain darf nicht mit einem Bindestrich enden",
    };
  }

  // Check for consecutive hyphens
  if (normalized.includes("--")) {
    return {
      valid: false,
      error: "Subdomain darf keine aufeinanderfolgenden Bindestriche enthalten",
    };
  }

  // Check against reserved slugs
  if (RESERVED_SLUGS.includes(normalized as ReservedSlug)) {
    return {
      valid: false,
      error: "Diese Subdomain ist reserviert und kann nicht verwendet werden",
    };
  }

  return { valid: true };
}

/**
 * Normalizes a slug by converting to lowercase and trimming whitespace.
 *
 * @param slug - The slug to normalize
 * @returns Normalized slug
 */
export function normalizeSlug(slug: string): string {
  return slug.toLowerCase().trim();
}

/**
 * Generates a suggested slug from an organization name.
 *
 * @param name - Organization name
 * @returns Suggested slug
 */
export function generateSlugFromName(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replaceAll(/[äÄ]/g, "ae")
    .replaceAll(/[öÖ]/g, "oe")
    .replaceAll(/[üÜ]/g, "ue")
    .replaceAll("ß", "ss")
    .replaceAll(/[^a-z0-9]+/g, "-") // Replace non-alphanumeric with hyphens
    .replaceAll(/(^-+)|(-+$)/g, "") // Remove leading/trailing hyphens
    .replaceAll(/-+/g, "-") // Replace multiple hyphens with single
    .slice(0, 30); // Limit length
}
