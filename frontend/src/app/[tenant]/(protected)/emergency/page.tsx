"use client";

import { useCallback, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Printer } from "lucide-react";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  exportEmergencySnapshot,
  type EmergencySnapshotExportMode,
} from "~/lib/emergency-export-api";
import { useEmergencyHealthInfoEnabled } from "~/lib/tenant-context";

export default function EmergencyPage() {
  const { status } = useSession({ required: true });
  const healthInfoOnList = useEmergencyHealthInfoEnabled();
  const [isExporting, setIsExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleExport = useCallback(
    async (mode: EmergencySnapshotExportMode) => {
      setIsExporting(true);
      setError(null);
      try {
        await exportEmergencySnapshot(mode);
      } catch {
        setError(
          "Die Notfallliste konnte nicht erstellt werden. Bitte versuchen Sie es erneut.",
        );
      } finally {
        setIsExporting(false);
      }
    },
    [],
  );
  const handlePrint = useCallback(() => {
    handleExport("print").catch(() => undefined);
  }, [handleExport]);
  const handleDownload = useCallback(() => {
    handleExport("download").catch(() => undefined);
  }, [handleExport]);

  return (
    <TenantPage
      title="Notfallliste"
      loading={status === "loading"}
      loadingLabel="Notfallliste wird geladen…"
      error={
        error
          ? {
              message: error,
              keepContent: true,
            }
          : null
      }
    >
      <SectionCard
        title="Notfallliste erstellen"
        description={
          <>
            Druckbare Liste aller Kinder, die gerade anwesend sind. Sie enthält
            Klasse, Ort oder Raum, Telefonnummern
            {healthInfoOnList
              ? ", Kontaktpersonen und die hinterlegten Gesundheitsinfos."
              : " und Kontaktpersonen."}
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

        {healthInfoOnList ? (
          <p className="mt-2 text-sm leading-6 text-gray-600">
            Steht bei einem Kind &bdquo;Nicht hinterlegt&ldquo;, sind keine
            Gesundheitsinfos eingetragen. Das heißt nicht, dass das Kind keine
            Allergie hat.
          </p>
        ) : null}
      </SectionCard>
    </TenantPage>
  );
}
