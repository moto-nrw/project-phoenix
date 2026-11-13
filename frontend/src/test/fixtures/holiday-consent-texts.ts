/**
 * The German consent texts of version 5 as the offer sends them (#5626,
 * #6691, #6692), with the retention of a year.
 */

import type { HolidayConsentTexts } from "~/lib/holiday-consent-texts";

export const HOLIDAY_CONSENT_TEXTS: HolidayConsentTexts = {
  version: 5,
  retentionDays: 365,
  privacyConsent:
    "Ich bin einverstanden, dass der Träger die Angaben für die Ferienbetreuung nutzt. Er löscht sie 365 Tage nach den Ferien. Gesundheitsangaben löscht er schon 92 Tage nach den Ferien. Bekommt meine Rechnung eine Kundennummer der Gemeinde (Debitorennummer), behält der Träger sie für spätere Anmeldungen. Er löscht sie am Ende des dritten Jahres nach der letzten Anmeldung. Muss er Rechnungen länger aufbewahren, löscht er sie danach.",
  healthConsent:
    "Ich bin einverstanden: Der Träger und die OGS sehen die Gesundheitsangaben meiner Kinder.",
  bindingConfirmed: "Ich weiß, dass die Anmeldung verbindlich ist.",
  custodyConfirmed:
    "Ich darf die Anmeldung allein abgeben: Ich bin allein sorgeberechtigt oder die andere sorgeberechtigte Person ist einverstanden.",
  declarationName: "Mit Ihrem Namen geben Sie die Erklärungen oben ab.",
};
