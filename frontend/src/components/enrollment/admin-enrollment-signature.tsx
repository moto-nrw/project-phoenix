"use client";

import { useLayoutEffect, useRef, useState } from "react";
import Image from "next/image";
import { Download } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { downloadAdminEnrollmentSignatureProof } from "~/lib/enrollment-admin-api";
import type { EnrollmentSignature } from "~/lib/enrollment-submission-api";

function signedAtLabel(value: string): string {
  return new Date(value).toLocaleString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * The parent's signature of the current version of an enrollment (#4786),
 * a simple electronic signature with a drawn signature, with the proof as
 * a PDF. Read-only: the signature belongs to the parents.
 */
export function AdminEnrollmentSignature({
  requestId,
  signature,
}: Readonly<{ requestId: string; signature: EnrollmentSignature }>) {
  const { show } = useApiErrorDisplay();
  const [downloading, setDownloading] = useState(false);

  const download = async () => {
    setDownloading(true);
    try {
      await downloadAdminEnrollmentSignatureProof(requestId);
    } catch (err: unknown) {
      void show(err, {
        object: "Der Anmeldenachweis",
        retry: () => void downloadRef.current(),
      });
    } finally {
      setDownloading(false);
    }
  };
  // The retry runs the latest download, not the one of the failed attempt.
  const downloadRef = useRef(download);
  useLayoutEffect(() => {
    downloadRef.current = download;
  });

  return (
    <SectionCard
      title="Unterschrift"
      description="Die Eltern haben diese Fassung der Anmeldung unterschrieben."
      bodyClassName="mt-4 space-y-4"
      actions={
        <Button
          type="button"
          variant="outline"
          size="md"
          className="gap-1.5"
          onClick={() => void download()}
          isLoading={downloading}
          loadingText="Nachweis wird geladen …"
          disabled={downloading}
        >
          <Download className="h-4 w-4" aria-hidden="true" />
          Nachweis als PDF
        </Button>
      }
    >
      {signature.image_png ? (
        <div className="rounded-xl bg-gray-50 p-2">
          {/* A stored PNG as data URL; next/image cannot optimise it. */}
          <Image
            src={`data:image/png;base64,${signature.image_png}`}
            alt={`Unterschrift von ${signature.signer_name}`}
            width={600}
            height={160}
            unoptimized
            className="h-24 w-full object-contain"
          />
        </div>
      ) : null}
      <DataGrid>
        <DataField label="Unterschrieben von">
          {signature.signer_name}
        </DataField>
        <DataField label="Unterschrieben am">
          {signedAtLabel(signature.signed_at)}
        </DataField>
        <DataField label="Art">
          {signature.method === "typed"
            ? `Name getippt: ${signature.typed_name ?? ""}`
            : "Mit Finger, Stift oder Maus gezeichnet"}
        </DataField>
        <DataField
          label="Prüfsumme der unterschriebenen Anmeldung"
          mono
          fullWidth
        >
          {signature.document_sha256}
        </DataField>
      </DataGrid>
      {signature.integrity_ok ? null : (
        <Alert
          type="warning"
          message="Achtung: Der gespeicherte Eintrag wurde nachträglich verändert."
        />
      )}
    </SectionCard>
  );
}
