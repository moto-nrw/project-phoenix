/**
 * The payment text of a Träger in the confirmation letter (#5622, ADR 0121
 * addendum): its own sentence with four fixed placeholders. The backend
 * fills them in (`letter_text.go`) and refuses any other text in curly
 * braces; this module checks a text before it is saved and shows the
 * sentence with example values.
 */

import { textTooLong } from "./text-length";

/** The Träger setting of the payment text. */
export const PAYMENT_TEXT_KEY = "parent_billing.payment_text";

/** The Träger switch that asks for the debtor number (#5621). */
export const DEBTOR_NUMBER_ON_CONFIRM_KEY =
  "parent_billing.debtor_number_on_confirm";

/** The settings tab of the holiday care of the Träger (#5621). */
export const TRAEGER_HOLIDAY_TAB = "ferien";

/** The longest payment text the setting takes. */
const PAYMENT_TEXT_MAX_LENGTH = 600;

const PAYMENT_TEXT_PLACEHOLDERS = [
  "{Betrag}",
  "{Zahlungsfrist}",
  "{Rechnungsnummer}",
  "{Debitorennummer}",
] as const;

/**
 * The built-in sentence of the letter, as the field shows it for an empty
 * text. With a debtor number the letter names it instead of the invoice
 * number (#5621).
 */
export const BUILT_IN_PAYMENT_TEXT =
  "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist} auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer {Rechnungsnummer} an.";

const BUILT_IN_PAYMENT_TEXT_WITH_DEBTOR_NUMBER =
  "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist} auf das Konto unten. Geben Sie als Verwendungszweck Ihre Debitorennummer {Debitorennummer} an.";

/** The example values of the preview: invented, no real family. */
const PAYMENT_TEXT_EXAMPLE: Readonly<
  Record<(typeof PAYMENT_TEXT_PLACEHOLDERS)[number], string>
> = {
  "{Betrag}": "124,00 €",
  "{Zahlungsfrist}": "15.12.2026",
  "{Rechnungsnummer}": "FE-2026-0007",
  "{Debitorennummer}": "10 4711",
};

const ALLOWED = new Set<string>(PAYMENT_TEXT_PLACEHOLDERS);
const ALLOWED_TEXT =
  "{Betrag}, {Zahlungsfrist}, {Rechnungsnummer} und {Debitorennummer}";

/**
 * Says what is wrong with a payment text before it is saved, in the words
 * of the field; null for a text the backend takes. An empty text keeps the
 * built-in sentence.
 */
export function paymentTextProblem(text: string): string | null {
  const tooLong = textTooLong(text, PAYMENT_TEXT_MAX_LENGTH);
  if (tooLong) return tooLong;
  let rest = text;
  for (const match of text.matchAll(/\{[^{}]*\}/g)) {
    if (!ALLOWED.has(match[0])) {
      return `„${match[0]}“ ist kein Platzhalter. Erlaubt sind ${ALLOWED_TEXT}.`;
    }
    rest = rest.replace(match[0], "");
  }
  if (rest.includes("{") || rest.includes("}")) {
    return "Eine geschweifte Klammer steht allein. Platzhalter stehen ganz in Klammern, zum Beispiel {Betrag}.";
  }
  return null;
}

/**
 * The payment sentence with the example values, as a letter would print
 * it. An empty text shows the built-in sentence; with the debtor number
 * switched on, it names the debtor number.
 */
export function paymentTextPreview(
  text: string,
  asksDebtorNumber: boolean,
  /** An example in the Träger's format (#6675); the default without one. */
  debtorNumberExample = "",
): string {
  const sentence =
    text.trim() ||
    (asksDebtorNumber
      ? BUILT_IN_PAYMENT_TEXT_WITH_DEBTOR_NUMBER
      : BUILT_IN_PAYMENT_TEXT);
  return PAYMENT_TEXT_PLACEHOLDERS.reduce(
    (result, placeholder) =>
      result.replaceAll(
        placeholder,
        placeholder === "{Debitorennummer}" && debtorNumberExample
          ? debtorNumberExample
          : PAYMENT_TEXT_EXAMPLE[placeholder],
      ),
    sentence,
  );
}
