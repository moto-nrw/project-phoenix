// Die Erklärung einer Ferienanmeldung im Träger-Portal (#5626): das
// Sorgerecht mit Zeitpunkt, der Name, mit dem die Eltern die Erklärungen
// abgegeben haben, die Fassung und der Wortlaut dieser Fassung.

import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { HolidayRegistrationDetail } from "~/lib/traeger-holiday-registrations-api";

type DeclarationRegistration = Pick<
  HolidayRegistrationDetail,
  "custodyConfirmedAt" | "declarationName" | "consentVersion" | "consentTexts"
>;

function versionText(version: number): string {
  return version > 0
    ? `Fassung ${version}`
    : "Vor Fassung 2, Wortlaut nicht gespeichert";
}

/** The declaration beside the consents in the parents' card. */
export function HolidayDeclarationFields({
  registration,
}: Readonly<{ registration: DeclarationRegistration }>) {
  return (
    <>
      <DataField label="Sorgerecht bestätigt">
        {registration.custodyConfirmedAt
          ? `Ja, am ${formatBerlinDate(registration.custodyConfirmedAt)}`
          : "Nein"}
      </DataField>
      <DataField label="Name als Erklärung">
        {registration.declarationName || "Nicht angegeben"}
      </DataField>
      <DataField label="Fassung der Texte">
        {versionText(registration.consentVersion)}
      </DataField>
    </>
  );
}

/** The wording of the version the parents agreed to, folded by default. */
export function HolidayDeclarationTextsCard({
  registration,
}: Readonly<{ registration: DeclarationRegistration }>) {
  const texts = registration.consentTexts;
  if (!texts) return null;
  const entries: readonly { label: string; text: string }[] = [
    { label: "Datenschutz", text: texts.privacyConsent },
    { label: "Gesundheitsangaben", text: texts.healthConsent },
    { label: "Verbindliche Anmeldung", text: texts.bindingConfirmed },
    { label: "Sorgerecht", text: texts.custodyConfirmed },
    { label: "Name als Erklärung", text: texts.declarationName },
  ].filter((entry) => entry.text !== "");
  return (
    <SectionCard
      title="Wortlaut der Erklärung"
      description={`Diese Texte standen bei der Anmeldung im Formular (${versionText(texts.version)}). Was die Eltern bestätigt haben, steht bei den Eltern.`}
      collapsible
      defaultCollapsed
    >
      <DataGrid>
        {entries.map((entry) => (
          <DataField key={entry.label} label={entry.label} fullWidth>
            {entry.text}
          </DataField>
        ))}
      </DataGrid>
    </SectionCard>
  );
}
