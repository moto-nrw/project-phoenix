/** The German consent texts of version 3 as the offer sends them (#5626). */

import type { HolidayConsentTexts } from "~/lib/holiday-registration-api";

export const HOLIDAY_CONSENT_TEXTS: HolidayConsentTexts = {
  version: 3,
  privacyConsent:
    "Ich bin einverstanden, dass der Träger die Angaben für die Ferienbetreuung nutzt. Er löscht sie drei Monate nach den Ferien.",
  healthConsent:
    "Ich bin einverstanden: Der Träger und die OGS sehen die Gesundheitsangaben meiner Kinder.",
  bindingConfirmed: "Ich weiß, dass die Anmeldung verbindlich ist.",
  custodyConfirmed:
    "Ich bin allein sorgeberechtigt. Oder die andere sorgeberechtigte Person ist mit der Anmeldung einverstanden.",
  declarationName: "Mit Ihrem Namen geben Sie die Erklärungen oben ab.",
};
