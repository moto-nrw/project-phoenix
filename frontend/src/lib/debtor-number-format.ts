// Das Format der Debitorennummer eines Trägers (#6675): mit „nur Ziffern“
// nimmt moto nur Nummern aus den Ziffern 0 bis 9 mit einer Länge von … bis …
// an. Dieselbe Regel prüft das Backend (`parentbilling.NormalizeDebtorNumber`);
// hier prüft das Feld vorab, damit das Büro den Fehler beim Tippen sieht. Das
// Backend bleibt maßgeblich.

/** The longest debtor number the backend keeps (#5621). */
export const DEBTOR_NUMBER_MAX_LENGTH = 35;

export interface DebtorNumberFormat {
  readonly digitsOnly: boolean;
  readonly minDigits: number;
  readonly maxDigits: number;
}

/** Without the setting every one-line number up to 35 characters is fine. */
export const NO_DEBTOR_NUMBER_FORMAT: DebtorNumberFormat = {
  digitsOnly: false,
  minDigits: 0,
  maxDigits: 0,
};

export interface BackendDebtorNumberFormat {
  readonly digits_only: boolean;
  readonly min_digits: number;
  readonly max_digits: number;
}

/** Maps the backend shape; a missing format means none. */
export function debtorNumberFormatOf(
  raw: BackendDebtorNumberFormat | null | undefined,
): DebtorNumberFormat {
  if (!raw?.digits_only) return NO_DEBTOR_NUMBER_FORMAT;
  return debtorNumberFormat(true, raw.min_digits, raw.max_digits);
}

const clampDigits = (digits: number) =>
  Math.max(1, Math.min(DEBTOR_NUMBER_MAX_LENGTH, Math.trunc(digits) || 1));

/**
 * The format from the Träger's three settings. The two lengths save one
 * after the other, so they count in either order, like in the backend.
 */
export function debtorNumberFormat(
  digitsOnly: boolean,
  first: number,
  second: number,
): DebtorNumberFormat {
  if (!digitsOnly) return NO_DEBTOR_NUMBER_FORMAT;
  const low = clampDigits(Math.min(first, second));
  const high = clampDigits(Math.max(first, second));
  return { digitsOnly: true, minDigits: low, maxDigits: high };
}

const EXAMPLE_DIGITS = "20815471136925814703692581470369258";

/** An example in the format: five digits where the range allows it. */
export function debtorNumberExample(format: DebtorNumberFormat): string {
  if (!format.digitsOnly) return "";
  const length = Math.max(format.minDigits, Math.min(5, format.maxDigits));
  return EXAMPLE_DIGITS.slice(0, length);
}

/** „5 Ziffern“ or „4 bis 6 Ziffern“. */
function debtorNumberDigitsText(format: DebtorNumberFormat): string {
  if (format.minDigits === format.maxDigits) {
    return `${format.minDigits} ${format.minDigits === 1 ? "Ziffer" : "Ziffern"}`;
  }
  return `${format.minDigits} bis ${format.maxDigits} Ziffern`;
}

/**
 * The helper text under a field when the Träger set a format, e.g. „Nur
 * Ziffern, genau 5, zum Beispiel 20815.“; empty without a format.
 */
export function debtorNumberFormatHint(format: DebtorNumberFormat): string {
  if (!format.digitsOnly) return "";
  return `Die Nummer hat ${debtorNumberDigitsText(format)}, zum Beispiel ${debtorNumberExample(format)}.`;
}

/**
 * The text of a number in another format, the same sentence the backend's
 * error code shows; null when the number fits or is empty.
 */
export function debtorNumberFormatError(
  value: string,
  format: DebtorNumberFormat,
): string | null {
  const number = value.trim();
  if (!format.digitsOnly || number === "") return null;
  const fits =
    /^[0-9]+$/.test(number) &&
    number.length >= format.minDigits &&
    number.length <= format.maxDigits;
  if (fits) return null;
  return `Die Debitorennummer hat ${debtorNumberDigitsText(format)}, zum Beispiel ${debtorNumberExample(format)}.`;
}
