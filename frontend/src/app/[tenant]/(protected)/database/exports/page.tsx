"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import Link from "next/link";
import {
  ArrowRight,
  Cake,
  ClipboardList,
  Clock,
  Download,
  DoorOpen,
  FileSpreadsheet,
  FileText,
  Printer,
  Users,
} from "lucide-react";

import { StudentExportModal } from "~/components/students/student-export-modal";
import { Button } from "~/components/ui/button";
import { InfoCard } from "~/components/ui/info-card";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { exportEmergencySnapshot } from "~/lib/emergency-export-api";
import {
  exportRoomSnapshot,
  type RoomSnapshotExportFormat,
} from "~/lib/room-export-api";
import type { StudentExportPreset } from "~/lib/student-export-api";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "DatabaseExportsPage" });

const ROOM_FORMATS: Array<{
  format: RoomSnapshotExportFormat;
  label: string;
  icon: ReactNode;
}> = [
  { format: "pdf", label: "PDF", icon: <FileText className="h-4 w-4" /> },
  { format: "docx", label: "DOCX", icon: <FileText className="h-4 w-4" /> },
  {
    format: "xlsx",
    label: "XLSX",
    icon: <FileSpreadsheet className="h-4 w-4" />,
  },
];

interface StudentModalConfig {
  heading: string;
  /** Omitted for the general-purpose child list, which offers every template. */
  lockedPreset?: StudentExportPreset;
  hiddenPresets?: readonly StudentExportPreset[];
}

/**
 * Central export entry point for the Datenverwaltung. It collects the exports
 * the individual pages already offer so they are findable in one place; it does
 * not reimplement them.
 *
 * Anmeldungen and Zeitnachweis are link cards rather than cards with their own
 * controls: both are scoped to a single entity (one enrollment phase, one staff
 * member) and have no "export everything" endpoint, so a picker here would only
 * duplicate the page that already does the job.
 */
export default function DatabaseExportsPage() {
  const isMobile = useIsMobile();
  const toast = useToast();
  const [studentModal, setStudentModal] = useState<StudentModalConfig | null>(
    null,
  );
  const [busy, setBusy] = useState<string | null>(null);

  const runExport = async (key: string, task: () => Promise<void>) => {
    setBusy(key);
    try {
      await task();
      toast.success("Export wurde erstellt.");
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Export fehlgeschlagen";
      logger.error("central_export_failed", { export: key, error: message });
      toast.error(message);
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="w-full">
      {isMobile && <PageHeaderWithSearch title="Exporte" />}

      <div className="min-h-[60vh] space-y-6">
        <p className="max-w-3xl text-sm text-gray-600">
          Alle Listen der Schule an einer Stelle. Jeder Export enthält nur die
          Daten, die für die jeweilige Liste nötig sind. Bitte behandle die
          erzeugten Dateien wie jede andere personenbezogene Unterlage.
        </p>

        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
          <InfoCard title="Kinderliste" icon={<Users className="h-5 w-5" />}>
            <ExportDescription>
              Wochen-, Klassen-, Tages- oder Abholliste. Vorlage, Format und
              Spalten sind frei wählbar.
            </ExportDescription>
            <ExportActions>
              <Button
                type="button"
                size="md"
                variant="outline"
                onClick={() =>
                  setStudentModal({
                    heading: "Kinderliste exportieren",
                    // The birthday list has its own card here, so offering it
                    // in the grid too would be a second route to one list.
                    hiddenPresets: ["birthday_list"],
                  })
                }
              >
                <Download className="mr-2 h-4 w-4" aria-hidden />
                Liste erstellen
              </Button>
            </ExportActions>
          </InfoCard>

          <InfoCard
            title="Geburtstagsliste"
            icon={<Cake className="h-5 w-5" />}
          >
            <ExportDescription>
              Geburtstage nach Kalender sortiert, wahlweise für einzelne Monate
              oder das ganze Jahr.
            </ExportDescription>
            <ExportActions>
              <Button
                type="button"
                size="md"
                variant="outline"
                onClick={() =>
                  setStudentModal({
                    heading: "Geburtstagsliste exportieren",
                    lockedPreset: "birthday_list",
                  })
                }
              >
                <Download className="mr-2 h-4 w-4" aria-hidden />
                Liste erstellen
              </Button>
            </ExportActions>
          </InfoCard>

          <InfoCard
            title="Notfallliste"
            icon={<ClipboardList className="h-5 w-5" />}
          >
            <ExportDescription>
              Alle aktuell anwesenden Kinder mit Kontaktdaten der
              Erziehungsberechtigten. Momentaufnahme.
            </ExportDescription>
            <ExportActions>
              <Button
                type="button"
                size="md"
                variant="outline"
                disabled={busy === "emergency-download"}
                onClick={() =>
                  void runExport("emergency-download", () =>
                    exportEmergencySnapshot("download"),
                  )
                }
              >
                <Download className="mr-2 h-4 w-4" aria-hidden />
                PDF
              </Button>
              <Button
                type="button"
                size="md"
                variant="outline"
                disabled={busy === "emergency-print"}
                onClick={() =>
                  void runExport("emergency-print", () =>
                    exportEmergencySnapshot("print"),
                  )
                }
              >
                <Printer className="mr-2 h-4 w-4" aria-hidden />
                Drucken
              </Button>
            </ExportActions>
          </InfoCard>

          <InfoCard title="Wer ist wo" icon={<DoorOpen className="h-5 w-5" />}>
            <ExportDescription>
              Aktuelle Belegung aller Räume mit Aufsicht und Kinderzahl.
              Momentaufnahme.
            </ExportDescription>
            <ExportActions>
              {ROOM_FORMATS.map(({ format, label, icon }) => (
                <Button
                  key={format}
                  type="button"
                  size="md"
                  variant="outline"
                  disabled={busy === `rooms-${format}`}
                  onClick={() =>
                    void runExport(`rooms-${format}`, () =>
                      exportRoomSnapshot({
                        format,
                        title: "Wer ist wo",
                        // room_ids stays omitted on purpose: every room. An
                        // empty array would select nothing and render an
                        // empty file.
                        include_transit: true,
                      }),
                    )
                  }
                >
                  <span className="mr-2">{icon}</span>
                  {label}
                </Button>
              ))}
            </ExportActions>
          </InfoCard>

          <InfoCard title="Anmeldungen" icon={<FileText className="h-5 w-5" />}>
            <ExportDescription>
              Eingegangene Anmeldungen einer Anmeldephase. Der Export gehört zur
              jeweiligen Phase.
            </ExportDescription>
            <ExportLink href="/admin/enrollments">
              Zu den Anmeldephasen
            </ExportLink>
          </InfoCard>

          <InfoCard title="Zeitnachweis" icon={<Clock className="h-5 w-5" />}>
            <ExportDescription>
              Arbeitszeiten einer Person für einen Zeitraum. Der Export gehört
              zum jeweiligen Profil.
            </ExportDescription>
            <ExportLink href="/database/personal">Zum Personal</ExportLink>
          </InfoCard>
        </div>
      </div>

      <StudentExportModal
        isOpen={studentModal !== null}
        filters={{}}
        heading={studentModal?.heading}
        lockedPreset={studentModal?.lockedPreset}
        hiddenPresets={studentModal?.hiddenPresets}
        onClose={() => setStudentModal(null)}
      />
    </div>
  );
}

function ExportDescription({ children }: Readonly<{ children: ReactNode }>) {
  return <p className="text-sm text-gray-600">{children}</p>;
}

function ExportActions({ children }: Readonly<{ children: ReactNode }>) {
  return <div className="flex flex-wrap gap-2 pt-1">{children}</div>;
}

function ExportLink({
  href,
  children,
}: Readonly<{ href: string; children: ReactNode }>) {
  const tenantPath = useTenantAwarePath();
  return (
    <Link
      href={tenantPath(href)}
      className="group inline-flex items-center pt-1 text-sm font-medium text-gray-700 transition-colors hover:text-gray-950"
    >
      {children}
      <ArrowRight
        className="ml-2 h-4 w-4 transition-transform group-hover:translate-x-0.5"
        aria-hidden
      />
    </Link>
  );
}
