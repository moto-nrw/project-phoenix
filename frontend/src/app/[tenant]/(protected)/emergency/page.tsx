"use client";

import { useCallback, useState } from "react";
import { useSession } from "next-auth/react";
import { AlertTriangle, Download, Printer } from "lucide-react";
import { Button } from "~/components/ui/button";
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

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  return (
    <main className="flex min-h-[calc(100dvh-11rem)] w-full items-center justify-center px-4 py-10 sm:px-6 lg:px-8">
      <section className="w-full max-w-3xl rounded-[32px] border border-gray-200 bg-white p-8 text-center shadow-[0_20px_70px_rgba(15,23,42,0.08),0_1px_2px_rgba(15,23,42,0.06)] sm:p-10 lg:p-12">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full bg-[#FF3130]/10 text-[#FF3130] ring-8 ring-[#FF3130]/5">
          <AlertTriangle className="h-8 w-8" aria-hidden />
        </div>

        <div className="mx-auto mt-6 max-w-2xl">
          <h1 className="text-3xl font-semibold tracking-normal text-gray-950 sm:text-4xl">
            Notfallliste
          </h1>
          <p className="mt-4 text-lg leading-8 text-gray-600">
            Druckbare Liste aller aktuell anwesenden Kinder mit Ort oder Raum,
            Klasse, Telefonnummern und Kontaktpersonen.
          </p>
        </div>

        {error ? (
          <div className="mt-8 rounded-xl border border-[#FF3130]/30 bg-[#FF3130]/10 p-4 text-left text-[#FF3130]">
            {error}
          </div>
        ) : null}

        <div className="mt-10 grid gap-3 sm:grid-cols-2">
          <Button
            type="button"
            variant="primary"
            size="xl"
            isLoading={isExporting}
            loadingText="Erstelle PDF..."
            onClick={() => void handleExport("print")}
            className="h-16 gap-3 rounded-xl"
          >
            <Printer className="h-5 w-5" aria-hidden />
            Notfallliste drucken
          </Button>
          <Button
            type="button"
            variant="outline"
            size="xl"
            disabled={isExporting}
            onClick={() => void handleExport("download")}
            className="h-16 gap-3 rounded-xl"
          >
            <Download className="h-5 w-5" aria-hidden />
            PDF herunterladen
          </Button>
        </div>

        <p className="mt-6 text-sm leading-6 text-gray-500">
          Die Liste wird beim Erstellen aus der aktuellen Anwesenheit erzeugt.
        </p>
      </section>
    </main>
  );
}
