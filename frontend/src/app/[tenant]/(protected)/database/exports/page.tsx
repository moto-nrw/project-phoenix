"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import Link from "next/link";
import { useSession } from "next-auth/react";
import {
  ArrowRight,
  Cake,
  CalendarCheck,
  CalendarRange,
  Download,
  FileSpreadsheet,
  FileText,
  Printer,
} from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

import { StudentExportModal } from "~/components/students/student-export-modal";
import { StaffBirthdayExportModal } from "~/components/staff/staff-birthday-export-modal";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";
import { Button } from "~/components/ui/button";
import { InfoCard } from "~/components/ui/info-card";
import { TenantPage } from "~/components/ui/tenant-page";
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
import { useEmergencyHealthInfoEnabled } from "~/lib/tenant-context";

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
  ogs_weekly: <MotoConceptIcon concept="calendar" size={20} />,
  ogs_compact: <MotoConceptIcon concept="lists" size={20} />,
  class_roster: <MotoConceptIcon concept="children" size={20} />,
  daily_planning: <MotoConceptIcon concept="carePlan" size={20} />,
  attendance_snapshot: <MotoConceptIcon concept="present" size={20} />,
  pickup_list: <MotoConceptIcon concept="pickup" size={20} />,
  blank_checklist: <MotoConceptIcon concept="activities" size={20} />,
  birthday_list: <MotoConceptIcon concept="birthdays" size={20} />,
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
  const toast = useToast();
  const { data: session, status } = useSession();
  // Slot lists are part of the timetable feature; hide the entry when a tenant
  // has explicitly disabled it (#1565 review), mirroring the sidebar and the
  // Betreuungsplan/Vertretung route guards. getSettingValue returns undefined
  // when the user cannot read settings, so the card stays visible by default.
  const { data: settingsSchema } = useSettingsSchema(
    status === "authenticated",
    { revalidateOnFocus: false, revalidateOnReconnect: false },
  );
  const timetableDisabled =
    getSettingValue(settingsSchema, "timetable.enabled") === false;
  // "Wer ist wo" hits POST /api/rooms/export, which requires rooms:read on top
  // of the users:read that gates this page. Without rooms:read the export 403s,
  // so hide the card rather than offer a button that always fails.
  const canReadRooms = isAdmin(session) || hasPermission(session, "rooms:read");
  // The Notfallliste card names what the PDF contains, so it has to know
  // whether this school prints the health column (#2609).
  const healthInfoOnEmergencyList = useEmergencyHealthInfoEnabled();
  // Der Dienstplan ist admin-only (die Seite leitet andere auf /staff um), der
  // Export folgt derselben Grenze statt auf eine Sackgasse zu verlinken.
  const canEditPlans = isAdmin(session);
  // Slot lists expose named children + presence, so the backend requires
  // schedules:read AND users:read (#1565), mirror that here. Der
  // Betreuungsplan-Export verlangt dieselbe Kombination.
  const canUseSlotLists =
    isAdmin(session) ||
    (hasPermission(session, "schedules:read") &&
      hasPermission(session, "users:read"));
  // Die Personal-Geburtstagsliste zeigt volle Geburtsdaten und hängt deshalb
  // an derselben Grenze wie die Stammdaten, aus denen sie stammt (#1542):
  // users:read reicht bewusst nicht.
  // Die Berechtigung heißt backendseitig `time_tracking:manage` mit
  // Unterstrich (permissions.ResourceTimeTracking); die Bindestrich-Variante
  // trifft niemanden und hätte die Karte für Leitungsrollen verschluckt.
  const canExportStaffBirthdays =
    isAdmin(session) ||
    hasPermission(session, "users:update") ||
    hasPermission(session, "time_tracking:manage");
  const [studentModal, setStudentModal] = useState<StudentModalConfig | null>(
    null,
  );
  const [staffBirthdayModalOpen, setStaffBirthdayModalOpen] = useState(false);
  // Every in-flight export, keyed per action: a single "which one is running"
  // value would let a finished export re-enable the buttons of one still
  // rendering, so two clicks could fire the same export twice.
  const [busy, setBusy] = useState<ReadonlySet<string>>(new Set());

  // Statuszeile des Seitenkopfs: was auf dieser Seite tatsächlich sichtbar
  // ist, also nach denselben Rechte-Schaltern wie die Karten selbst.
  const listCount =
    STUDENT_EXPORT_PRESETS.length +
    (canExportStaffBirthdays ? 1 : 0) +
    1 + // Notfallliste
    (canReadRooms ? 1 : 0);
  const linkCount =
    (canUseSlotLists && !timetableDisabled ? 1 : 0) +
    (canEditPlans && !timetableDisabled ? 1 : 0) +
    (canUseSlotLists && !timetableDisabled ? 1 : 0) +
    (isAdmin(session) ? 1 : 0) +
    1; // Zeitnachweis
  const statusLine = `${listCount} ${listCount === 1 ? "Liste" : "Listen"} · ${linkCount} auf anderen Seiten`;

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
    <TenantPage title="Exporte" stats={statusLine} back>
      <div className="min-h-[60vh] space-y-6">
        <p className="text-sm text-gray-600">
          Jeder Export enthält nur die Daten, die für die jeweilige Liste nötig
          sind. Bitte behandeln Sie die erzeugten Dateien wie jede andere
          personenbezogene Unterlage.
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

        {canExportStaffBirthdays && (
          <ExportSection title="Personallisten">
            <InfoCard
              title="Geburtstagsliste"
              icon={<Cake className="h-5 w-5" />}
            >
              <ExportDescription>
                Geburtstage der Mitarbeitenden nach Kalender sortiert.
                Voreingestellt ist der aktuelle Monat. Ohne hinterlegtes
                Geburtsdatum fehlt eine Person in dieser Liste.
              </ExportDescription>
              <ExportActions>
                <Button
                  type="button"
                  size="md"
                  variant="outline"
                  onClick={() => setStaffBirthdayModalOpen(true)}
                >
                  <Download className="mr-2 h-4 w-4" aria-hidden />
                  Liste erstellen
                </Button>
              </ExportActions>
            </InfoCard>
          </ExportSection>
        )}

        <ExportSection title="Momentaufnahmen">
          <InfoCard
            title="Notfallliste"
            icon={<MotoConceptIcon concept="emergency" size={20} />}
          >
            <ExportDescription>
              Alle aktuell anwesenden Kinder mit Kontaktdaten der
              Erziehungsberechtigten
              {healthInfoOnEmergencyList
                ? " und den hinterlegten Gesundheitsinfos"
                : ""}
              . Momentaufnahme.
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
              icon={<MotoConceptIcon concept="rooms" size={20} />}
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

        <ExportSection title="Auf anderen Seiten">
          {canUseSlotLists && !timetableDisabled && (
            <InfoCard
              title="Tageslisten"
              icon={<MotoConceptIcon concept="lists" size={20} />}
            >
              <ExportDescription>
                Listen aus geplanten Angeboten wie Mensa, Lernzeit, AG oder
                Ganztag: Plan, Ist und Abgleich für ein Datum. Der Einstieg
                liegt im Bereich Planung.
              </ExportDescription>
              <ExportLink href="/lists">Zu den Tageslisten</ExportLink>
            </InfoCard>
          )}
          {/* Die beiden Wochenpläne (#2079) exportieren immer die Woche, die
              auf ihrer Seite gerade zu sehen ist. Ein Datumsdialog hier wäre
              eine zweite, konkurrierende Bedienung derselben Sache. */}
          {canEditPlans && !timetableDisabled && (
            <InfoCard
              title="Dienstplan"
              icon={<CalendarRange className="h-5 w-5" />}
            >
              <ExportDescription>
                Die Dienstplanwoche zum Aushängen: wer wann wo arbeitet,
                wahlweise nach Personen oder nach Einsatzbereich. Der Export
                liegt im Dienstplan.
              </ExportDescription>
              <ExportLink href="/dienstplan">Zum Dienstplan</ExportLink>
            </InfoCard>
          )}
          {canUseSlotLists && !timetableDisabled && (
            <InfoCard
              title="Betreuungsplan"
              icon={<CalendarCheck className="h-5 w-5" />}
            >
              <ExportDescription>
                Die Betreuungswoche mit Zeiten, Räumen, Personal und Kinderzahl.
                Der Export liegt im Betreuungsplan.
              </ExportDescription>
              <ExportLink href="/betreuungsplan">Zum Betreuungsplan</ExportLink>
            </InfoCard>
          )}
          {/* /admin/enrollments redirects non-admins to /dashboard (useRequireAdmin),
              so only offer the link to admins rather than send others to a dead end. */}
          {isAdmin(session) && (
            <InfoCard
              title="Anmeldungen"
              icon={<MotoConceptIcon concept="enrollments" size={20} />}
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

          <InfoCard
            title="Zeitnachweis"
            icon={<MotoConceptIcon concept="timeTracking" size={20} />}
          >
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

      <StaffBirthdayExportModal
        isOpen={staffBirthdayModalOpen}
        onClose={() => setStaffBirthdayModalOpen(false)}
      />
    </TenantPage>
  );
}

function ExportSection({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <section className="space-y-3">
      <h2 className="text-base font-semibold text-gray-900">{title}</h2>
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
