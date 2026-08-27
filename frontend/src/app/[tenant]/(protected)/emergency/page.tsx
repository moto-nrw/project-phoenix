"use client";

import { useCallback, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Printer } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  exportEmergencySnapshot,
  type EmergencySnapshotExportMode,
} from "~/lib/emergency-export-api";
import { useEmergencyHealthInfoEnabled } from "~/lib/tenant-context";
import { formatStatusDate } from "~/lib/date-helpers";

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
      stats={formatStatusDate()}
      loading={status === "loading"}
    >
      {/* Die beiden Knöpfe bleiben groß und mittig: im Notfall zählt das
          Ziel, nicht die Dichte. Das Gerüst darüber ist dasselbe wie auf
          jeder anderen Seite. */}
      <SectionCard
        title="Liste jetzt erstellen"
        description={
          <>
            Druckbare Liste aller Kinder, die gerade anwesend sind. Sie enthält
            Klasse, Ort oder Raum, Telefonnummern
            {healthInfoOnList
              ? ", Kontaktpersonen und die hinterlegten Gesundheitsinfos."
              : " und Kontaktpersonen."}{" "}
            Die Liste wird beim Erstellen aus der aktuellen Anwesenheit erzeugt.
          </>
        }
        leading={
          <span className="bg-moto-red/10 text-moto-red flex h-10 w-10 shrink-0 items-center justify-center rounded-xl">
            <MotoConceptIcon concept="emergency" size={20} />
          </span>
        }
      >
        <div className="space-y-4">
          {error ? <Alert type="error" message={error} /> : null}

          <div className="grid gap-3 sm:grid-cols-2">
            <Button
              type="button"
              variant="primary"
              size="xl"
              isLoading={isExporting}
              loadingText="Erstelle PDF…"
              onClick={handlePrint}
              className="h-14 gap-3"
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
              className="h-14 gap-3"
            >
              <Download className="h-5 w-5" aria-hidden />
              PDF herunterladen
            </Button>
          </div>

          {healthInfoOnList ? (
            <p className="text-sm leading-6 text-gray-500">
              Steht bei einem Kind „Nicht hinterlegt", sind keine
              Gesundheitsinfos eingetragen. Das heißt nicht, dass das Kind keine
              Allergie hat.
            </p>
          ) : null}
        </div>
      </SectionCard>
    </TenantPage>
  );
}
