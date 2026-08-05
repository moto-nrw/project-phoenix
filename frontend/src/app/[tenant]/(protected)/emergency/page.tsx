"use client";

import { useCallback, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Printer } from "lucide-react";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  exportEmergencySnapshot,
  type EmergencySnapshotExportMode,
} from "~/lib/emergency-export-api";
import { Loading } from "~/components/ui/loading";

export default function EmergencyPage() {
  const { status } = useSession({ required: true });
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

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  return (
    <main className="flex w-full items-center justify-center px-4 py-6 sm:min-h-[calc(100dvh-11rem)] sm:px-6 sm:py-10 lg:px-8">
      <section className="w-full max-w-3xl rounded-[32px] border border-gray-200 bg-white p-6 text-center shadow-[0_20px_70px_rgba(15,23,42,0.08),0_1px_2px_rgba(15,23,42,0.06)] sm:p-10 lg:p-12">
        <div className="bg-moto-red/10 text-moto-red ring-moto-red/5 mx-auto flex h-14 w-14 items-center justify-center rounded-full ring-8 sm:h-16 sm:w-16">
          <MotoConceptIcon
            concept="emergency"
            size={32}
            className="h-7 w-7 sm:h-8 sm:w-8"
          />
        </div>

        <div className="mx-auto mt-4 max-w-2xl sm:mt-6">
          <h1 className="text-2xl font-semibold tracking-normal text-gray-950 sm:text-4xl">
            Notfallliste
          </h1>
          <p className="mt-3 text-base leading-7 text-gray-600 sm:mt-4 sm:text-lg sm:leading-8">
            Druckbare Liste aller aktuell anwesenden Kinder mit Ort oder Raum,
            Klasse, Telefonnummern und Kontaktpersonen.
          </p>
        </div>

        {error ? (
          <div className="border-moto-red/30 bg-moto-red/10 text-moto-red mt-8 rounded-xl border p-4 text-left">
            {error}
          </div>
        ) : null}

        <div className="mt-6 grid gap-3 sm:mt-10 sm:grid-cols-2">
          <Button
            type="button"
            variant="primary"
            size="xl"
            isLoading={isExporting}
            loadingText="Erstelle PDF..."
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

        <p className="mt-4 text-sm leading-6 text-gray-500 sm:mt-6">
          Die Liste wird beim Erstellen aus der aktuellen Anwesenheit erzeugt.
        </p>
      </section>
    </main>
  );
}
