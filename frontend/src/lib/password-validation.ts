/**
 * Shared password validation utilities.
 * Used by signup forms and invitation acceptance flows.
 */

export interface PasswordRequirement {
  label: string;
  test: (value: string) => boolean;
}

/**
 * Password requirements for account creation.
 * All requirements must be met for a valid password.
 */
export const PASSWORD_REQUIREMENTS: PasswordRequirement[] = [
  { label: "Mindestens 8 Zeichen", test: (value) => value.length >= 8 },
  { label: "Ein Großbuchstabe", test: (value) => /[A-Z]/.test(value) },
  { label: "Ein Kleinbuchstabe", test: (value) => /[a-z]/.test(value) },
  { label: "Eine Zahl", test: (value) => /\d/.test(value) },
  { label: "Ein Sonderzeichen", test: (value) => /[^A-Za-z0-9]/.test(value) },
];

/**
 * Check if a password meets all requirements.
 */
export function validatePassword(password: string): boolean {
  return PASSWORD_REQUIREMENTS.every((req) => req.test(password));
}

/**
 * Get the status of each password requirement.
 */
export function getPasswordRequirementStatus(
  password: string,
): Array<{ label: string; met: boolean }> {
  return PASSWORD_REQUIREMENTS.map(({ label, test }) => ({
    label,
    met: test(password),
  }));
}
