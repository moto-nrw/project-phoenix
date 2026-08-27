"use client";

import { useCallback, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Printer } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
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

        <section className="moto-content-surface rounded-2xl border p-4 text-center shadow-sm sm:p-6">
          <div className="bg-moto-red/10 text-moto-red ring-moto-red/5 mx-auto flex h-14 w-14 items-center justify-center rounded-full ring-8 sm:h-16 sm:w-16">
            <MotoConceptIcon
              concept="emergency"
              size={32}
              className="h-7 w-7 sm:h-8 sm:w-8"
            />
          </div>

          <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-gray-600">
            Druckbare Liste aller Kinder, die gerade anwesend sind. Sie enthält
            Klasse, Ort oder Raum, Telefonnummern
            {healthInfoOnList
              ? ", Kontaktpersonen und die hinterlegten Gesundheitsinfos."
              : " und Kontaktpersonen."}
          </p>

          <div className="mx-auto mt-6 grid max-w-2xl gap-3 sm:grid-cols-2">
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

          <p className="mt-4 text-sm leading-6 text-gray-500">
            Die Liste wird beim Erstellen aus der aktuellen Anwesenheit erzeugt.
          </p>

          {healthInfoOnList ? (
            <p className="mt-2 text-sm leading-6 text-gray-500">
              Steht bei einem Kind &bdquo;Nicht hinterlegt&ldquo;, sind keine
              Gesundheitsinfos eingetragen. Das heißt nicht, dass das Kind keine
              Allergie hat.
            </p>
          ) : null}
        </section>
      </div>
    </div>
  );
}
