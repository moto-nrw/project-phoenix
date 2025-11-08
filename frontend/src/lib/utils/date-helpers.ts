/**
 * Date utility functions for validating and formatting dates
 */

/**
 * Checks if a date string represents a valid date
 * @param dateString - The date string to validate
 * @returns true if the date is valid, false otherwise
 */
export function isValidDateString(
  dateString: string | null | undefined,
): boolean {
  if (!dateString) return false;
  const date = new Date(dateString);
  return !isNaN(date.getTime());
}

/**
 * Checks if a date has expired (is in the past)
 * @param dateString - The date string to check
 * @returns true if the date is in the past, false otherwise or if invalid
 */
export function isDateExpired(dateString: string | null | undefined): boolean {
  if (!isValidDateString(dateString)) return false;
  const date = new Date(dateString!);
  return date.getTime() < Date.now();
}

/**
 * Safely parses a date string and returns a Date object or null
 * @param dateString - The date string to parse
 * @returns Date object if valid, null otherwise
 */
export function safeParseDate(
  dateString: string | null | undefined,
): Date | null {
  if (!isValidDateString(dateString)) return null;
  return new Date(dateString!);
}
