// Nachweis für den Leistungsbezug einer Ferienanmeldung (#5238): die Dateien,
// die Eltern mit der Anmeldung hochgeladen haben. Nur die Einzelansicht zeigt
// sie, nie die Liste oder der Export. Jede Datei öffnet sich in einem neuen
// Tab; das Backend protokolliert jedes Öffnen.

import { ExternalLink } from "lucide-react";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { SectionCard } from "~/components/ui/section-card";
import { formatUploadSize } from "~/lib/enrollment-uploads";
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
        <ul className="space-y-1.5 text-sm">
          {proofs.map((proof) => (
            <li
              key={proof.id}
              className="flex flex-wrap items-center gap-x-2 gap-y-1"
            >
              <FileTypeIcon
                contentType={proof.contentType}
                className="h-4 w-4 shrink-0"
              />
              <span className="min-w-0 font-medium break-all text-gray-900">
                {proof.filename}
              </span>
              <span className="text-gray-500">
                {formatUploadSize(proof.sizeBytes)}
              </span>
              <a
                href={holidayProofHref(periodId, registration.id, proof.id)}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 font-medium text-gray-900 underline underline-offset-2 hover:text-gray-600"
              >
                Öffnen
                <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                <span className="sr-only">{proof.filename} in neuem Tab</span>
              </a>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-gray-700">
          Die Eltern haben keinen Nachweis hochgeladen. Bitte fordern Sie ihn
          bei der Familie an.
        </p>
      )}
    </SectionCard>
  );
}
