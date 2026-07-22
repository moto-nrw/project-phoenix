"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import Link from "next/link";
import { useSession } from "next-auth/react";
import {
  ArrowRight,
  Cake,
  CalendarClock,
  CalendarDays,
  Car,
  ClipboardList,
  Clock,
  Download,
  DoorOpen,
  FileSpreadsheet,
  FileText,
  GraduationCap,
  LayoutList,
  ListChecks,
  Printer,
  UserCheck,
} from "lucide-react";

import { StudentExportModal } from "~/components/students/student-export-modal";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
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
import {
  STUDENT_EXPORT_PRESETS,
  type StudentExportPreset,
} from "~/lib/student-export-api";
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
  lockedPreset: StudentExportPreset;
}

/**
 * One icon per child-list template. Each list is its own card here, so the
 * template picker inside the modal only appears in the Kindersuche.
 */
const STUDENT_LIST_ICONS: Record<StudentExportPreset, ReactNode> = {
  ogs_weekly: <CalendarDays className="h-5 w-5" />,
  ogs_compact: <LayoutList className="h-5 w-5" />,
  class_roster: <GraduationCap className="h-5 w-5" />,
  daily_planning: <CalendarClock className="h-5 w-5" />,
  attendance_snapshot: <UserCheck className="h-5 w-5" />,
  pickup_list: <Car className="h-5 w-5" />,
  blank_checklist: <ListChecks className="h-5 w-5" />,
  birthday_list: <Cake className="h-5 w-5" />,
};

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
  const { data: session } = useSession();
  // "Wer ist wo" hits POST /api/rooms/export, which requires rooms:read on top
  // of the users:read that gates this page. Without rooms:read the export 403s,
  // so hide the card rather than offer a button that always fails.
  const canReadRooms = isAdmin(session) || hasPermission(session, "rooms:read");
  // Slot lists expose named children + presence, so the backend requires
  // schedules:read AND users:read (#1565) — mirror that here.
  const canUseSlotLists =
    isAdmin(session) ||
    (hasPermission(session, "schedules:read") &&
      hasPermission(session, "users:read"));
  const [studentModal, setStudentModal] = useState<StudentModalConfig | null>(
    null,
  );
  // Every in-flight export, keyed per action: a single "which one is running"
  // value would let a finished export re-enable the buttons of one still
  // rendering, so two clicks could fire the same export twice.
  const [busy, setBusy] = useState<ReadonlySet<string>>(new Set());

  const runExport = async (key: string, task: () => Promise<void>) => {
    setBusy((current) => new Set(current).add(key));
    try {
      await task();
      toast.success("Export wurde erstellt.");
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Export fehlgeschlagen";
      logger.error("central_export_failed", { export: key, error: message });
      toast.error(message);
    } finally {
      setBusy((current) => {
        const next = new Set(current);
        next.delete(key);
        return next;
      });
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

        <ExportSection title="Kinderlisten">
          {STUDENT_EXPORT_PRESETS.map((preset) => (
            <InfoCard
              key={preset.id}
              title={preset.label}
              icon={STUDENT_LIST_ICONS[preset.id]}
            >
              <ExportDescription>{preset.description}</ExportDescription>
              <ExportActions>
                <Button
                  type="button"
                  size="md"
                  variant="outline"
                  onClick={() =>
                    setStudentModal({
                      heading: `${preset.label} exportieren`,
                      lockedPreset: preset.id,
                    })
                  }
                >
                  <Download className="mr-2 h-4 w-4" aria-hidden />
                  Liste erstellen
                </Button>
              </ExportActions>
            </InfoCard>
          ))}
        </ExportSection>

        <ExportSection title="Momentaufnahmen">
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
                disabled={busy.has("emergency-download")}
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
                disabled={busy.has("emergency-print")}
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

          {canReadRooms && (
            <InfoCard
              title="Wer ist wo"
              icon={<DoorOpen className="h-5 w-5" />}
            >
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
                    disabled={busy.has(`rooms-${format}`)}
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
          )}
        </ExportSection>

        {canUseSlotLists && (
          <ExportSection title="Betreuungsplan">
            <InfoCard
              title="Tageslisten"
              icon={<CalendarClock className="h-5 w-5" />}
            >
              <ExportDescription>
                Listen aus geplanten Angeboten wie Mensa, Lernzeit, AG oder
                Ganztag: Plan, Ist und Abgleich für ein Datum — mit Vorschau,
                Zählern und PDF-/XLSX-Export.
              </ExportDescription>
              <ExportLink href="/lists">Zu den Tageslisten</ExportLink>
            </InfoCard>
          </ExportSection>
        )}

        <ExportSection title="Auf anderen Seiten">
          {/* /admin/enrollments redirects non-admins to /dashboard (useRequireAdmin),
              so only offer the link to admins rather than send others to a dead end. */}
          {isAdmin(session) && (
            <InfoCard
              title="Anmeldungen"
              icon={<FileText className="h-5 w-5" />}
            >
              <ExportDescription>
                Eingegangene Anmeldungen einer Anmeldephase. Der Export gehört
                zur jeweiligen Phase.
              </ExportDescription>
              <ExportLink href="/admin/enrollments">
                Zu den Anmeldephasen
              </ExportLink>
            </InfoCard>
          )}

          <InfoCard title="Zeitnachweis" icon={<Clock className="h-5 w-5" />}>
            <ExportDescription>
              Arbeitszeiten einer Person für einen Zeitraum. Der Export gehört
              zum jeweiligen Profil.
            </ExportDescription>
            <ExportLink href="/database/personal">Zum Personal</ExportLink>
          </InfoCard>
        </ExportSection>
      </div>

      <StudentExportModal
        isOpen={studentModal !== null}
        filters={{}}
        heading={studentModal?.heading}
        lockedPreset={studentModal?.lockedPreset}
        onClose={() => setStudentModal(null)}
      />
    </div>
  );
}

function ExportSection({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <section className="space-y-3">
      <h2 className="text-base font-semibold text-gray-950">{title}</h2>
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {children}
      </div>
    </section>
  );
}

function ExportDescription({ children }: Readonly<{ children: ReactNode }>) {
  return <p className="text-sm text-gray-600">{children}</p>;
}

function ExportActions({ children }: Readonly<{ children: ReactNode }>) {
  // mt-auto pins the action row to the card bottom so buttons line up across a
  // row regardless of how long each card's description runs.
  return <div className="mt-auto flex flex-wrap gap-2 pt-1">{children}</div>;
}

function ExportLink({
  href,
  children,
}: Readonly<{ href: string; children: ReactNode }>) {
  const tenantPath = useTenantAwarePath();
  return (
    <Link
      href={tenantPath(href)}
      className="group mt-auto inline-flex items-center pt-1 text-sm font-medium text-gray-700 transition-colors hover:text-gray-950"
    >
      {children}
      <ArrowRight
        className="ml-2 h-4 w-4 transition-transform group-hover:translate-x-0.5"
        aria-hidden
      />
    </Link>
  );
}
