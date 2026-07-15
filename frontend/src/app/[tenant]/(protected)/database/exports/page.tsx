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
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useToast } from "~/contexts/ToastContext";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { exportEmergencySnapshot } from "~/lib/emergency-export-api";
import {
  exportRoomSnapshot,
  type RoomSnapshotExportFormat,
} from "~/lib/room-export-api";
import type { StudentExportPreset } from "~/lib/student-export-api";

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
  preset: StudentExportPreset;
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
          <ExportCard
            icon={<Users className="h-6 w-6" />}
            iconColor={LOCATION_COLORS.OTHER_ROOM}
            title="Kinderliste"
            description="Wochen-, Klassen-, Tages- oder Abholliste. Vorlage, Format und Spalten sind frei wählbar."
          >
            <Button
              type="button"
              size="md"
              variant="secondary"
              onClick={() =>
                setStudentModal({
                  heading: "Kinderliste exportieren",
                  preset: "ogs_weekly",
                })
              }
            >
              <Download className="h-4 w-4" aria-hidden />
              Liste erstellen
            </Button>
          </ExportCard>

          <ExportCard
            icon={<Cake className="h-6 w-6" />}
            iconColor={LOCATION_COLORS.SICK}
            title="Geburtstagsliste"
            description="Geburtstage nach Kalender sortiert, wahlweise für einzelne Monate oder das ganze Jahr."
          >
            <Button
              type="button"
              size="md"
              variant="secondary"
              onClick={() =>
                setStudentModal({
                  heading: "Geburtstagsliste exportieren",
                  preset: "birthday_list",
                })
              }
            >
              <Download className="h-4 w-4" aria-hidden />
              Liste erstellen
            </Button>
          </ExportCard>

          <ExportCard
            icon={<ClipboardList className="h-6 w-6" />}
            iconColor={LOCATION_COLORS.HOME}
            title="Notfallliste"
            description="Alle aktuell anwesenden Kinder mit Kontaktdaten der Erziehungsberechtigten. Momentaufnahme."
          >
            <Button
              type="button"
              size="md"
              variant="secondary"
              disabled={busy === "emergency-download"}
              onClick={() =>
                void runExport("emergency-download", () =>
                  exportEmergencySnapshot("download"),
                )
              }
            >
              <Download className="h-4 w-4" aria-hidden />
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
              <Printer className="h-4 w-4" aria-hidden />
              Drucken
            </Button>
          </ExportCard>

          <ExportCard
            icon={<DoorOpen className="h-6 w-6" />}
            iconColor={LOCATION_COLORS.TRANSIT}
            title="Wer ist wo"
            description="Aktuelle Belegung aller Räume mit Aufsicht und Kinderzahl. Momentaufnahme."
          >
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
                      // room_ids stays omitted on purpose: every room. An empty
                      // array would select nothing and render an empty file.
                      include_transit: true,
                    }),
                  )
                }
              >
                {icon}
                {label}
              </Button>
            ))}
          </ExportCard>

          <ExportLinkCard
            icon={<FileText className="h-6 w-6" />}
            iconColor={LOCATION_COLORS.GROUP_ROOM}
            title="Anmeldungen"
            description="Eingegangene Anmeldungen einer Anmeldephase. Der Export gehört zur jeweiligen Phase."
            href="/admin/enrollments"
            cta="Zu den Anmeldephasen"
          />

          <ExportLinkCard
            icon={<Clock className="h-6 w-6" />}
            iconColor={LOCATION_COLORS.SCHOOLYARD}
            title="Zeitnachweis"
            description="Arbeitszeiten einer Person für einen Zeitraum. Der Export gehört zum jeweiligen Profil."
            href="/database/personal"
            cta="Zum Personal"
          />
        </div>
      </div>

      <StudentExportModal
        isOpen={studentModal !== null}
        filters={{}}
        heading={studentModal?.heading}
        initialPreset={studentModal?.preset}
        onClose={() => setStudentModal(null)}
      />
    </div>
  );
}

function CardIcon({
  icon,
  iconColor,
}: Readonly<{ icon: ReactNode; iconColor: string }>) {
  return (
    <div
      className="mb-4 w-fit rounded-2xl p-3 text-white shadow-sm"
      style={{ backgroundColor: iconColor }}
    >
      {icon}
    </div>
  );
}

function ExportCard({
  icon,
  iconColor,
  title,
  description,
  children,
}: Readonly<{
  icon: ReactNode;
  iconColor: string;
  title: string;
  description: string;
  children: ReactNode;
}>) {
  return (
    <div className="moto-content-surface flex flex-col rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
      <CardIcon icon={icon} iconColor={iconColor} />
      <h3 className="mb-2 text-lg font-bold text-gray-900">{title}</h3>
      <p className="mb-4 flex-1 text-sm text-gray-600">{description}</p>
      <div className="flex flex-wrap gap-2">{children}</div>
    </div>
  );
}

function ExportLinkCard({
  icon,
  iconColor,
  title,
  description,
  href,
  cta,
}: Readonly<{
  icon: ReactNode;
  iconColor: string;
  title: string;
  description: string;
  href: string;
  cta: string;
}>) {
  return (
    <Link
      href={href}
      className="moto-content-surface moto-hover-elevated group flex touch-manipulation flex-col rounded-2xl border border-gray-200 bg-white p-6 shadow-sm"
    >
      <CardIcon icon={icon} iconColor={iconColor} />
      <h3 className="mb-2 text-lg font-bold text-gray-900">{title}</h3>
      <p className="mb-4 flex-1 text-sm text-gray-600">{description}</p>
      <span className="flex items-center text-sm font-medium text-gray-400 transition-colors group-hover:text-gray-700">
        {cta}
        <ArrowRight className="ml-2 h-4 w-4 transition-transform group-hover:translate-x-0.5" />
      </span>
    </Link>
  );
}
