// Nachweis für den Leistungsbezug einer Ferienanmeldung (#5238): die Dateien,
// die Eltern mit der Anmeldung hochgeladen haben. Nur die Einzelansicht zeigt
// sie, nie die Liste oder der Export. Jede Datei öffnet sich in einem neuen
// Tab; das Backend protokolliert jedes Öffnen.

import { AttachmentList } from "~/components/ui/attachment-list";
import { SectionCard } from "~/components/ui/section-card";
import {
  holidayProofHref,
  type HolidayRegistrationDetail,
} from "~/lib/traeger-holiday-registrations-api";

export function HolidayRegistrationProofCard({
  periodId,
  registration,
}: Readonly<{
  periodId: string;
  registration: HolidayRegistrationDetail;
}>) {
  const proofs = registration.benefitsProofs;
  if (!registration.benefitsDeclared && proofs.length === 0) return null;

  return (
    <SectionCard title="Nachweis für den Leistungsbezug">
      {proofs.length > 0 ? (
        <AttachmentList
          attachments={proofs.map((proof) => ({
            id: proof.id,
            filename: proof.filename,
            size_bytes: proof.sizeBytes,
            content_type: proof.contentType,
          }))}
          downloadUrl={(proofId) =>
            holidayProofHref(periodId, registration.id, proofId)
          }
          openLabel="In neuem Tab öffnen"
        />
      ) : (
        <p className="text-sm text-gray-700">
          Die Eltern haben keinen Nachweis hochgeladen. Bitte fordern Sie ihn
          bei der Familie an.
        </p>
      )}
    </SectionCard>
  );
}
