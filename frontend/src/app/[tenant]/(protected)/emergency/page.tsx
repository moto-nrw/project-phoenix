"use client";

import { useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Printer } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
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

  // Begruendete Ausnahme zur Kopf-Aktion: im Notfall zaehlt ein Ziel, nicht
  // die Dichte. Deshalb steht „Notfallliste drucken" als einzige sichtbare
  // Aktion neben dem Titel, waehrend das PDF wie auf jeder anderen Flaeche im
  // Kebab liegt. Zwei gleichrangige Knoepfe im Inhalt gibt es hier nicht mehr.
  const menuItems = useMemo(
    () => [
      {
        label: "PDF herunterladen",
        icon: <Download className="h-4 w-4" aria-hidden />,
        onClick: handleDownload,
        disabled: isExporting,
      },
    ],
    [handleDownload, isExporting],
  );

  return (
    <TenantPage
      title="Notfallliste"
      stats={formatStatusDate()}
      loading={status === "loading"}
      actions={
        <>
          <Button
            type="button"
            variant="primary"
            size="md"
            isLoading={isExporting}
            loadingText="Erstelle PDF…"
            onClick={handlePrint}
            className="gap-2"
          >
            <Printer className="h-4 w-4" aria-hidden />
            Notfallliste drucken
          </Button>
          <OverflowMenu items={menuItems} ariaLabel="Weitere Aktionen" />
        </>
      }
    >
      <SectionCard
        title="Was auf der Liste steht"
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
