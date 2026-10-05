/**
 * Answer-option bounds and editing helpers for a parent poll (Umfrage).
 *
 * MAX_POLL_OPTIONS mirrors maxPollOptions in the backend
 * (modules/communication/internal/staffannouncements/service.go). The bound
 * fits a Terminabstimmung for an Elternsprechtag with 40 to 50 slots (#3861).
 */
export const MIN_POLL_OPTIONS = 2;
export const MAX_POLL_OPTIONS = 60;
export const MAX_POLL_OPTION_LENGTH = 120;

/**
 * Above this many options the parent portal shows the compact answer list. It
 * is the bound polls had before #3861, so every older poll looks unchanged.
 */
export const LONG_POLL_OPTIONS = 10;

/**
 * Splits pasted text into answer rows: one non-blank line per answer. Tabs
 * become spaces, because a row copied from a spreadsheet carries its columns
 * tab-separated ("Di 14.10.\t15:00").
 */
export function splitPastedOptions(text: string): string[] {
  return text
    .split(/\r\n|\r|\n/)
    .map((line) => line.replace(/\t+/g, " ").replace(/\s+/g, " ").trim())
    .filter(Boolean);
}

/**
 * Inserts pasted lines into the option rows at `index`.
 *
 * Returns null for single-line text, so the browser pastes it into the field
 * as usual. An empty row takes the first line; a row that already has text
 * keeps it and the lines follow below. Lines past MAX_POLL_OPTIONS or longer
 * than MAX_POLL_OPTION_LENGTH are dropped and counted, so the form can say so.
 */
export function insertPastedOptions(
  rows: readonly string[],
  index: number,
  text: string,
): { rows: string[]; dropped: number; tooLong: number } | null {
  const lines = splitPastedOptions(text);
  if (lines.length < 2) return null;

  const current = rows[index] ?? "";
  const replaceCurrent = current.trim() === "";
  const before = rows.slice(0, replaceCurrent ? index : index + 1);
  const after = rows.slice(index + 1);
  // Only filled rows count against the limit: blank rows are dropped on save.
  const filled = [...before, ...after].filter((row) => row.trim()).length;
  const validLines = lines.filter(
    // Spread iterates Unicode code points, like the backend's []rune check.
    (line) => [...line].length <= MAX_POLL_OPTION_LENGTH,
  );
  const room = Math.max(0, MAX_POLL_OPTIONS - filled);
  const taken = validLines.slice(0, room);
  return {
    rows: [...before, ...taken, ...after],
    dropped: validLines.length - taken.length,
    tooLong: lines.length - validLines.length,
  };
}
