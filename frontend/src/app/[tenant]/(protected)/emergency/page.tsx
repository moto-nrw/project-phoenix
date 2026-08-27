"use client";

import { useCallback, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Printer } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SectionCard } from "~/components/ui/section-card";
import {
  exportEmergencySnapshot,
  type EmergencySnapshotExportMode,
} from "~/lib/emergency-export-api";
import { SkeletonRegion, CardSkeleton } from "~/components/ui/page-skeletons";
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

  // Der Kopf rendert immer sofort; nur die Datenregion wird skeletonisiert.
  if (status === "loading") {
    return (
      <div className="-mt-1.5 w-full">
        <PageHeaderWithSearch title="Notfall" />
        <SkeletonRegion label="Notfallliste wird geladen…">
          <CardSkeleton rows={2} />
        </SkeletonRegion>
      </div>
    );
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* Der Seitentitel steht auf dem Desktop in der Breadcrumb der
          Kopfzeile; PageHeaderWithSearch blendet seine Überschrift ab md aus. */}
      <PageHeaderWithSearch title="Notfall" />

      <div className="space-y-6">
        {error ? <Alert type="error" message={error} /> : null}

        {/* Erklärtexte als description der Karte, der Gesundheitshinweis als
            Alert. Die beiden großen Aktionen bleiben große Touch-Ziele, stehen
            aber in einer Zeile im Kartenkörper. */}
        <SectionCard
          title="Notfallliste"
          description={`Druckbare Liste aller Kinder, die gerade anwesend sind. Sie enthält Klasse, Ort oder Raum, Telefonnummern${
            healthInfoOnList
              ? ", Kontaktpersonen und die hinterlegten Gesundheitsinfos."
              : " und Kontaktpersonen."
          } Die Liste wird beim Erstellen aus der aktuellen Anwesenheit erzeugt.`}
          leading={
            <span className="bg-moto-red/10 text-moto-red ring-moto-red/5 flex h-12 w-12 shrink-0 items-center justify-center rounded-full ring-8">
              <MotoConceptIcon
                concept="emergency"
                size={28}
                className="h-6 w-6"
              />
            </span>
          }
        >
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <Button
              type="button"
              variant="primary"
              size="xl"
              isLoading={isExporting}
              loadingText="Erstelle PDF…"
              onClick={handlePrint}
              className="h-14 gap-3 rounded-xl sm:h-16"
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
              className="h-14 gap-3 rounded-xl sm:h-16"
            >
              <Download className="h-5 w-5" aria-hidden />
              PDF herunterladen
            </Button>
          </div>

          {healthInfoOnList ? (
            <div className="mt-4">
              <Alert
                type="info"
                message="Steht bei einem Kind „Nicht hinterlegt“, sind keine Gesundheitsinfos eingetragen. Das heißt nicht, dass das Kind keine Allergie hat."
              />
            </div>
          ) : null}
        </SectionCard>
      </div>
    </div>
  );
}
