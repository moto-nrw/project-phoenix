"use client";

import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Printer } from "lucide-react";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  exportEmergencySnapshot,
  type EmergencySnapshotExportMode,
} from "~/lib/emergency-export-api";

export default function EmergencyPage() {
  const { status } = useSession({ required: true });
  const [isExporting, setIsExporting] = useState(false);
  const { show: showError } = useApiErrorDisplay();
  // „Wiederholen“ erstellt die Liste noch einmal auf demselben Weg.
  const retryRef = useRef<(mode: EmergencySnapshotExportMode) => void>(
    () => undefined,
  );

  const handleExport = useCallback(
    async (mode: EmergencySnapshotExportMode) => {
      setIsExporting(true);
      try {
        await exportEmergencySnapshot(mode);
      } catch (exportError) {
        void showError(exportError, {
          object: "die Notfallliste",
          retry: () => retryRef.current(mode),
        });
      } finally {
        setIsExporting(false);
      }
    },
    [showError],
  );
  useLayoutEffect(() => {
    retryRef.current = (mode) => void handleExport(mode);
  });
  const handlePrint = useCallback(() => {
    void handleExport("print");
  }, [handleExport]);
  const handleDownload = useCallback(() => {
    void handleExport("download");
  }, [handleExport]);

  return (
    <TenantPage
      title="Notfallliste"
      loading={status === "loading"}
      loadingLabel="Notfallliste wird geladen…"
    >
      <SectionCard
        title="Notfallliste erstellen"
        description={
          <>
            Druckbare Liste aller Kinder, die gerade anwesend sind. Sie enthält
            Klasse, Ort oder Raum, Telefonnummern, Kontaktpersonen und die
            hinterlegten Gesundheitsinfos.
          </>
        }
        leading={
          <div className="bg-moto-red/10 text-moto-red flex h-10 w-10 shrink-0 items-center justify-center rounded-xl">
            <MotoConceptIcon concept="emergency" size={24} />
          </div>
        }
      >
        <div className="grid gap-3 sm:grid-cols-2">
          <Button
            type="button"
            variant="primary"
            size="xl"
            isLoading={isExporting}
            loadingText="Erstelle PDF..."
            onClick={handlePrint}
            className="gap-3"
          >
            <Printer className="h-5 w-5" aria-hidden />
            Notfallliste drucken
          </Button>
          <Button
            type="button"
            variant="outline"
            size="xl"
            disabled={isExporting}
            onClick={handleDownload}
            className="gap-3"
          >
            <Download className="h-5 w-5" aria-hidden />
            PDF herunterladen
          </Button>
        </div>

        <p className="mt-4 text-sm leading-6 text-gray-600">
          Die Liste wird beim Erstellen aus der aktuellen Anwesenheit erzeugt.
        </p>

        <p className="mt-2 text-sm leading-6 text-gray-600">
          Steht bei einem Kind &bdquo;Nicht hinterlegt&ldquo;, sind keine
          Gesundheitsinfos eingetragen. Das heißt nicht, dass das Kind keine
          Allergie hat.
        </p>
      </SectionCard>
    </TenantPage>
  );
}
