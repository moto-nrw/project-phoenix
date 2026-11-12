"use client";

// Nachweis für den Leistungsbezug einer Ferienanmeldung (#5238): die Dateien,
// die Eltern mit der Anmeldung hochgeladen haben. Bringt die Familie den
// Nachweis auf Papier, lädt das Träger-Büro ihn hier hoch (#5627). Nur die
// Einzelansicht zeigt die Nachweise, nie die Liste oder der Export. Jede
// Datei lässt sich in einem neuen Tab öffnen und herunterladen; das Backend
// protokolliert jeden Abruf und jedes Hochladen.

import { Upload } from "lucide-react";
import { useRef, useState } from "react";
import { AttachmentList } from "~/components/ui/attachment-list";
import { Button } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  holidayProofHref,
  type HolidayRegistrationDetail,
  uploadHolidayProof,
} from "~/lib/traeger-holiday-registrations-api";

const logger = createLogger({ component: "HolidayRegistrationProofCard" });

function emptyText(registration: HolidayRegistrationDetail): string {
  return registration.benefitsDeclared
    ? "Die Eltern haben keinen Nachweis hochgeladen. Bitte fordern Sie ihn bei der Familie an. Bringt die Familie ihn auf Papier, laden Sie ihn hier hoch."
    : "Kein Nachweis. Bringt eine Familie mit Leistungsbezug den Nachweis auf Papier, laden Sie ihn hier hoch.";
}

export function HolidayRegistrationProofCard({
  periodId,
  registration,
  onUploaded,
}: Readonly<{
  periodId: string;
  registration: HolidayRegistrationDetail;
  /** After an upload, with the registration and its proofs. */
  onUploaded?: (registration: HolidayRegistrationDetail) => void;
}>) {
  const [uploading, setUploading] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const { show: showActionError } = useApiErrorDisplay();
  const { success } = useToast();
  const proofs = registration.benefitsProofs;
  const canUpload = Boolean(onUploaded) && registration.status !== "cancelled";
  if (!canUpload && !registration.benefitsDeclared && proofs.length === 0) {
    return null;
  }

  const upload = async (file: File) => {
    setUploading(true);
    try {
      const next = await uploadHolidayProof(periodId, registration.id, file);
      success("Der Nachweis ist hochgeladen.");
      onUploaded?.(next);
    } catch (err) {
      logger.error("holiday_proof_upload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showActionError(err, { object: "der Nachweis" });
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  };

  return (
    <SectionCard
      title="Nachweis für den Leistungsbezug"
      actions={
        canUpload ? (
          <>
            <input
              ref={fileRef}
              id="holiday-proof-upload"
              type="file"
              accept="application/pdf,image/jpeg,image/png,.pdf,.jpg,.jpeg,.png"
              className="sr-only"
              tabIndex={-1}
              aria-hidden="true"
              onChange={(event) => {
                const file = event.target.files?.[0];
                if (file) void upload(file);
              }}
            />
            <Button
              type="button"
              variant="outline"
              size="md"
              className="gap-2"
              isLoading={uploading}
              loadingText="Lädt hoch …"
              tooltip="Wählt eine PDF oder ein Foto und lädt es sofort als Nachweis hoch."
              onClick={() => fileRef.current?.click()}
            >
              <Upload className="h-4 w-4" aria-hidden="true" />
              Nachweis hochladen
            </Button>
          </>
        ) : null
      }
    >
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
          fileActions
        />
      ) : (
        <p className="text-sm text-gray-700">{emptyText(registration)}</p>
      )}
      {canUpload ? (
        <p className="mt-3 text-sm text-gray-600">
          PDF, JPEG oder PNG, höchstens 10 MB. Für die Kostenübernahme ist ein
          Nachweis keine Pflicht.
        </p>
      ) : null}
    </SectionCard>
  );
}
