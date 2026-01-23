/**
 * ReDoS-safe email validation utilities.
 *
 * Uses string methods instead of regex to prevent catastrophic backtracking.
 * Based on recommendations from:
 * - https://www.geeksforgeeks.org/how-to-validate-email-address-without-using-regular-expression-in-javascript/
 * - https://javascript.info/regexp-catastrophic-backtracking
 *
 * Note: This is intentionally permissive - it catches obvious formatting errors
 * while allowing most valid email addresses through. The only true validation
 * is sending an email and checking if it bounces.
 */

/**
 * Maximum allowed email length per RFC 5321
 */
export const MAX_EMAIL_LENGTH = 254;

/**
 * Validates an email address using string methods (no regex).
 * This approach is immune to ReDoS attacks.
 *
 * Checks:
 * - Not empty
 * - Within length limit (254 chars per RFC 5321)
 * - Contains exactly one @ symbol
 * - @ is not at the start or end
 * - Has a dot after the @ symbol
 * - Dot is not immediately after @
 * - Dot is not at the very end
 * - No spaces in the email
 *
 * @param email - The email address to validate
 * @returns true if the email appears valid, false otherwise
 */
export function isValidEmail(email: string): boolean {
  // Trim and check for empty
  const trimmed = email.trim();
  if (!trimmed) {
    return false;
  }

  // Check length limit (RFC 5321)
  if (trimmed.length > MAX_EMAIL_LENGTH) {
    return false;
  }

  // No spaces allowed
  if (trimmed.includes(" ")) {
    return false;
  }

  // Find @ position
  const atIndex = trimmed.indexOf("@");

  // Must have @ and not at start
  if (atIndex <= 0) {
    return false;
  }

  // @ must not be at the end
  if (atIndex === trimmed.length - 1) {
    return false;
  }

  // Must have exactly one @
  if (trimmed.includes("@", atIndex + 1)) {
    return false;
  }

  // Get local part and domain
  const localPart = trimmed.substring(0, atIndex);
  const domain = trimmed.substring(atIndex + 1);

  // Local part must not be empty
  if (localPart.length === 0) {
    return false;
  }

  // Domain must not be empty
  if (domain.length === 0) {
    return false;
  }

  // Domain must have at least one dot
  const dotIndex = domain.lastIndexOf(".");
  if (dotIndex === -1) {
    return false;
  }

  // Dot must not be at start of domain
  if (dotIndex === 0) {
    return false;
  }

  // Dot must not be at end of domain
  if (dotIndex === domain.length - 1) {
    return false;
  }

  // TLD must be at least 2 characters
  const tld = domain.substring(dotIndex + 1);
  if (tld.length < 2) {
    return false;
  }

  return true;
}

/**
 * Validates email with detailed error message.
 *
 * @param email - The email address to validate
 * @returns Object with valid boolean and optional error message
 */
export function validateEmail(email: string): {
  valid: boolean;
  error?: string;
} {
  const trimmed = email.trim();

  if (!trimmed) {
    return { valid: false, error: "E-Mail-Adresse ist erforderlich" };
  }

  if (trimmed.length > MAX_EMAIL_LENGTH) {
    return {
      valid: false,
      error: `E-Mail-Adresse darf maximal ${MAX_EMAIL_LENGTH} Zeichen haben`,
    };
  }

  if (trimmed.includes(" ")) {
    return {
      valid: false,
      error: "E-Mail-Adresse darf keine Leerzeichen enthalten",
    };
  }

  const atIndex = trimmed.indexOf("@");

  if (atIndex === -1) {
    return { valid: false, error: "E-Mail-Adresse muss ein @ enthalten" };
  }

  if (atIndex === 0) {
    return {
      valid: false,
      error: "E-Mail-Adresse darf nicht mit @ beginnen",
    };
  }

  if (atIndex === trimmed.length - 1) {
    return { valid: false, error: "E-Mail-Adresse darf nicht mit @ enden" };
  }

  if (trimmed.includes("@", atIndex + 1)) {
    return {
      valid: false,
      error: "E-Mail-Adresse darf nur ein @ enthalten",
    };
  }

  const domain = trimmed.substring(atIndex + 1);
  const dotIndex = domain.lastIndexOf(".");

  if (dotIndex === -1 || dotIndex === 0 || dotIndex === domain.length - 1) {
    return { valid: false, error: "Ungültige Domain in der E-Mail-Adresse" };
  }

  const tld = domain.substring(dotIndex + 1);
  if (tld.length < 2) {
    return { valid: false, error: "Ungültige Domain-Endung" };
  }

  return { valid: true };
}
