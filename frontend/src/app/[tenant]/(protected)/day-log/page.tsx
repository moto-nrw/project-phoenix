"use client";

// Tagesauswertung (#1456): per group and calendar day, every child with one
// day verdict: Anwesend / Krank / Klassenfahrt / Entschuldigt / Nicht
// eingeplant / Abwesend. Read-only evaluation of what NFC/web check-in and
// sick notes already record; gated by gdpr.attendance_log_enabled.
//
// Design follows the Anmeldungen/Planung surface language: one calm content
// section with an uppercase kicker, gray-50 stat blocks instead of colored
// chips, white per-group cards in a two-column grid, click opens the
// per-group detail modal with the status sections and group-scoped exports.

import { ArrowRight, Download, FileSpreadsheet, Printer } from "lucide-react";
import Link from "~/components/ui/navigation-link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuEntry } from "~/components/ui/page-header/OverflowMenu";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  DAY_LOG_STATUS_COLORS,
  DAY_LOG_STATUS_ORDER,
  DayLogError,
  dayLogExportUrl,
  dayLogSourceLabel,
  fetchDayLog,
  type DayLogErrorCode,
  type DayLogGroup,
  type DayLogResponse,
  type DayLogStatus,
  type DayLogStudent,
} from "~/lib/day-log-api";
import {
  berlinTodayISO,
  formatDate,
  formatStatusDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "DayLogPage" });

const ERROR_MESSAGES: Record<DayLogErrorCode, string> = {
  feature_disabled:
    "Das Anwesenheitsprotokoll ist für Ihre Schule nicht eingeschaltet. Ihre Leitung kann es in den Einstellungen unter Datenschutz einschalten.",
  not_group_supervisor:
    "Ihr Konto ist keinem Personaleintrag zugeordnet. Bitte wenden Sie sich an Ihre Administration.",
  no_permitted_groups:
    "Ihr Konto ist keinem Personaleintrag zugeordnet. Bitte wenden Sie sich an Ihre Administration.",
  invalid_request:
    "Die Tagesauswertung ist nur für den aktuellen Tag verfügbar.",
  unknown: "Die Tagesauswertung konnte nicht geladen werden.",
};

const STATUS_LABELS: Record<DayLogStatus, string> = {
  present: "Anwesend",
  sick: "Krank",
  class_trip: "Klassenfahrt",
  excused: "Entschuldigt",
  not_scheduled: "Nicht eingeplant",
  absent: "Unentschuldigt",
};

const MODAL_SECTION_TITLES: Record<DayLogStatus, string> = {
  present: "Anwesend",
  sick: "Krank",
  class_trip: "Klassenfahrt",
  excused: "Entschuldigt / Abgemeldet",
  not_scheduled: "Nicht eingeplant",
  absent: "Unentschuldigt abwesend",
};

type ExportFormat = "pdf" | "xlsx";

// Ladefehler gehören in den Fehlerzustand des Gerüsts. Ausgeschaltete Funktion
// und fehlender Personaleintrag sind Zustände und werden als Leerzustand mit
// dem nächsten Schritt gezeigt.
function isLoadError(code: DayLogErrorCode): boolean {
  return code === "unknown" || code === "invalid_request";
}

function counterFor(
  counters: DayLogGroup["counters"],
  status: DayLogStatus,
): number {
  switch (status) {
    case "present":
      return counters.present;
    case "sick":
      return counters.sick;
    case "class_trip":
      return counters.class_trip;
    case "excused":
      return counters.excused;
    case "not_scheduled":
      return counters.not_scheduled;
    default:
      return counters.absent;
  }
}

function formatClock(iso?: string): string {
  if (!iso) return "";
  const parsed = new Date(iso);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toLocaleTimeString("de-DE", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Berlin",
  });
}

function studentDetailLine(student: DayLogStudent): string {
  if (student.status === "present") {
    const from = formatClock(student.check_in_time);
    const until = student.check_out_time
      ? formatClock(student.check_out_time)
      : "";
    const range = until
      ? `Ankunft ${from} · gegangen ${until}`
      : `Ankunft ${from} · noch anwesend`;
    return student.hint ? `${range} · ${student.hint}` : range;
  }
  const parts: string[] = [];
  if (student.reported_at) {
    parts.push(`gemeldet ${formatClock(student.reported_at)} Uhr`);
  }
  const source = dayLogSourceLabel(student.source);
  if (source) parts.push(source);
  return parts.join(" · ");
}

// Stat is the calm gray value block used across the Anmeldungen section
// (PhaseStat recipe): value on top, muted label below, no chart-style color.
function Stat({
  label,
  value,
  highlight = false,
}: Readonly<{ label: string; value: number; highlight?: boolean }>) {
  return (
    <div className="rounded-xl bg-gray-50 px-3 py-2">
      <span
        className={`block text-sm font-semibold ${
          highlight && value > 0 ? "text-moto-red-strong" : "text-gray-900"
        }`}
      >
        {value}
      </span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}

function GroupCard({
  group,
  onOpen,
}: {
  readonly group: DayLogGroup;
  readonly onOpen: () => void;
}) {
  const c = group.counters;
  return (
    <SectionCard>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-gray-900">
            {group.name}
          </h3>
          <p className="mt-1 text-xs text-gray-500">
            {c.present} von {c.total} Kindern anwesend
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={onOpen}
          className="shrink-0 gap-2 bg-white"
        >
          Details
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
      <div className="mt-4 grid grid-cols-3 gap-2">
        {DAY_LOG_STATUS_ORDER.map((status) => (
          <Stat
            key={status}
            label={STATUS_LABELS[status]}
            value={counterFor(c, status)}
            highlight={status === "absent"}
          />
        ))}
      </div>
    </SectionCard>
  );
}

function StudentRow({ student }: { readonly student: DayLogStudent }) {
  const tenantPath = useTenantAwarePath();
  const detail = studentDetailLine(student);
  return (
    <li>
      <Link
        href={tenantPath(`/students/${student.student_id}?from=/day-log`)}
        className="flex items-center justify-between gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-gray-50"
      >
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-gray-900">
            {student.last_name}, {student.first_name}
          </p>
          <p className="truncate text-xs text-gray-500">
            {student.school_class}
            {detail ? ` · ${detail}` : ""}
          </p>
        </div>
        <StatusDotBadge
          label={student.label}
          color={DAY_LOG_STATUS_COLORS[student.status]}
        />
      </Link>
    </li>
  );
}

function GroupDetail({ group }: { readonly group: DayLogGroup }) {
  const sections = DAY_LOG_STATUS_ORDER.map((status) => ({
    status,
    students: group.students.filter((student) => student.status === status),
  })).filter((section) => section.students.length > 0);

  if (sections.length === 0) {
    return (
      <p className="py-4 text-sm text-gray-500">
        Keine Kinder in dieser Gruppe.
      </p>
    );
  }
  return (
    <div className="space-y-4">
      {sections.map((section) => (
        <div key={section.status}>
          <h3
            className={`mb-1 text-xs font-semibold tracking-wide uppercase ${
              section.status === "absent"
                ? "text-moto-red-strong"
                : "text-gray-500"
            }`}
          >
            {MODAL_SECTION_TITLES[section.status]} ({section.students.length})
          </h3>
          <ul className="divide-y divide-gray-100">
            {section.students.map((student) => (
              <StudentRow key={student.student_id} student={student} />
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}

export default function DayLogPage() {
  // Berlin, not browser-local: the backend validates against
  // timezone.TodayDate() (Europe/Berlin), so a browser in another timezone
  // must not default to a day the server considers future or already past.
  const now = useMinuteClock();
  const todayISO = berlinTodayISO(now);
  const [dateISO, setDateISO] = useState<string>(() => berlinTodayISO());
  const [data, setData] = useState<DayLogResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorCode, setErrorCode] = useState<DayLogErrorCode | null>(null);
  const [openGroupId, setOpenGroupId] = useState<string | null>(null);
  const [exporting, setExporting] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  // Keep an open page on the Berlin calendar day. The backend only accepts
  // today's roster, so a tab spanning midnight must refetch rather than retain
  // yesterday's now-invalid date.
  useEffect(() => {
    setDateISO((selected) => (selected === todayISO ? selected : todayISO));
  }, [todayISO]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setErrorCode(null);
    fetchDayLog(dateISO)
      .then((response) => {
        if (!cancelled) setData(response);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setData(null);
        setErrorCode(error instanceof DayLogError ? error.code : "unknown");
        logger.error("day_log_fetch_failed", {
          date: dateISO,
          error: error instanceof Error ? error.message : String(error),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [dateISO]);

  const openGroup = useMemo(
    () => data?.groups.find((group) => group.group_id === openGroupId) ?? null,
    [data, openGroupId],
  );

  // Group membership has no dated source of truth. Keep the selector on the
  // current day so a later transfer cannot relabel historical attendance.
  const minDate = useMemo(() => parseISODate(todayISO), [todayISO]);
  const maxDate = useMemo(() => parseISODate(todayISO), [todayISO]);

  const downloadExport = useCallback(
    async (format: ExportFormat, groupId?: string) => {
      setExporting(`${format}-${groupId ?? "all"}`);
      setExportError(null);
      try {
        const res = await fetch(dayLogExportUrl(dateISO, format, groupId));
        if (!res.ok) {
          setExportError("Export fehlgeschlagen. Bitte erneut versuchen.");
          return;
        }
        const blob = await res.blob();
        const disposition = res.headers.get("Content-Disposition") ?? "";
        const filename =
          /filename="([^"]+)"/.exec(disposition)?.[1] ??
          `tagesauswertung-${dateISO}.${format}`;
        const url = URL.createObjectURL(blob);
        const anchor = document.createElement("a");
        anchor.href = url;
        anchor.download = filename;
        document.body.appendChild(anchor);
        anchor.click();
        anchor.remove();
        URL.revokeObjectURL(url);
      } catch (error) {
        logger.error("day_log_export_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setExportError("Export fehlgeschlagen. Bitte erneut versuchen.");
      } finally {
        setExporting(null);
      }
    },
    [dateISO],
  );

  // Drucken: open the PDF in a new tab (blob URL, so the browser's viewer
  // shows it instead of forcing the attachment download) and print from there.
  const printPdf = useCallback(
    async (groupId?: string) => {
      // The tab must open synchronously inside the click gesture; popup
      // blockers discard windows opened after an await. Navigate it to the
      // blob URL once the PDF arrives; close it again if the export fails.
      // (window.open with the "noopener" feature returns null, so the opener
      // is severed manually instead.)
      const tab = window.open("", "_blank");
      if (tab) tab.opener = null;
      setExporting(`print-${groupId ?? "all"}`);
      setExportError(null);
      try {
        const res = await fetch(dayLogExportUrl(dateISO, "pdf", groupId));
        if (!res.ok) {
          tab?.close();
          setExportError(
            "Druckansicht fehlgeschlagen. Bitte erneut versuchen.",
          );
          return;
        }
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        if (tab) {
          tab.location.href = url;
        } else {
          // Popup blocked even in the gesture; last resort, may be blocked
          // too, but the blob URL stays valid for a manual retry.
          window.open(url, "_blank");
        }
        // Give the new tab time to load before releasing the object URL.
        setTimeout(() => URL.revokeObjectURL(url), 60_000);
      } catch (error) {
        tab?.close();
        logger.error("day_log_print_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setExportError("Druckansicht fehlgeschlagen. Bitte erneut versuchen.");
      } finally {
        setExporting(null);
      }
    },
    [dateISO],
  );

  // Eine sichtbare Aktion neben dem Titel, alles Weitere im Menü: Drucken ist
  // der tägliche Griff, PDF und Excel sind der seltene. Vorher standen alle
  // drei nebeneinander und füllten die halbe Kopfzeile -- dieselbe Reihe, die
  // die Statistik hatte.
  const exportMenuItems = (groupId?: string): OverflowMenuEntry[] => [
    { kind: "header", label: "Herunterladen" },
    {
      label:
        exporting === `pdf-${groupId ?? "all"}` ? "Wird exportiert…" : "PDF",
      icon: <Download className="h-4 w-4" aria-hidden />,
      onClick: () => void downloadExport("pdf", groupId),
      disabled: !data || exporting !== null,
    },
    {
      label:
        exporting === `xlsx-${groupId ?? "all"}` ? "Wird exportiert…" : "Excel",
      icon: <FileSpreadsheet className="h-4 w-4" aria-hidden />,
      onClick: () => void downloadExport("xlsx", groupId),
      disabled: !data || exporting !== null,
    },
  ];

  const exportButtons = (groupId?: string) => (
    <>
      <Button
        type="button"
        variant="primary"
        size="md"
        className="gap-2"
        disabled={!data || exporting !== null}
        onClick={() => void printPdf(groupId)}
      >
        <Printer className="h-4 w-4" aria-hidden />
        {exporting === `print-${groupId ?? "all"}`
          ? "Wird geöffnet…"
          : "Drucken"}
      </Button>
      <OverflowMenu
        items={exportMenuItems(groupId)}
        ariaLabel="Weitere Aktionen"
      />
    </>
  );

  const selectedDate = parseISODate(dateISO);

  // Statuszeile unter dem Titel: gewählter Tag und die Zahl der Kinder aus dem
  // geladenen Protokoll.
  const statusLine = data
    ? `${formatStatusDate(dateISO)} · ${data.counters.total} Kinder`
    : formatStatusDate(dateISO);

  return (
    <TenantPage
      title="Tagesauswertung"
      stats={statusLine}
      statsLoading={loading}
      loading={loading}
      // Ein Ladefehler ist `error`. Eine ausgeschaltete Funktion oder ein
      // fehlender Personaleintrag ist dagegen ein Zustand, kein Fehler: er
      // steht als Leerzustand mit dem nächsten Schritt.
      error={
        errorCode !== null && isLoadError(errorCode)
          ? ERROR_MESSAGES[errorCode]
          : null
      }
      empty={
        errorCode !== null && !isLoadError(errorCode)
          ? {
              title:
                errorCode === "feature_disabled"
                  ? "Anwesenheitsprotokoll ist ausgeschaltet"
                  : "Noch keine Gruppen für Ihr Konto",
              description: ERROR_MESSAGES[errorCode],
            }
          : errorCode === null && data !== null && data.groups.length === 0
            ? {
                title: "Keine Gruppe für diesen Tag",
                description:
                  "Für den gewählten Tag ist keine Gruppe sichtbar. Legen Sie eine Gruppe an oder lassen Sie sich einer Gruppe zuordnen.",
              }
            : null
      }
      actions={
        // Kein eigener Umbruch-Wrapper: wie die Aktionen auf dem Telefon
        // stehen, entscheidet die Kopfkarte für alle Seiten gleich.
        <>
          {/* Kit-picker call-site pattern from the datepicker sweep (#2016):
              w-44 trigger, field-aligned popover panel. Once the sweep
              lands, switch to ISODatePicker + controlSize="md". */}
          <DatePicker
            value={selectedDate}
            onChange={(date) => {
              if (date) setDateISO(toISODate(date));
            }}
            minDate={minDate}
            maxDate={maxDate}
            calendarLayout="popover"
            hideClearButton
            className="w-44"
          />
          {exportButtons()}
        </>
      }
      overlays={
        <>
          {/* Die Gruppe steht im Panel neben der Tagesliste: die Zahlen der
              anderen Gruppen bleiben dabei sichtbar. */}
          <SlideOver
            open={openGroup !== null}
            onOpenChange={(open) => {
              if (!open) setOpenGroupId(null);
            }}
          >
            <SlideOverContent widthClass="sm:w-[760px]">
              <SlideOverHeader className="flex-row items-start justify-between gap-3">
                <div className="min-w-0">
                  <SlideOverTitle>
                    {openGroup
                      ? `${openGroup.name} · ${formatDate(dateISO)}`
                      : ""}
                  </SlideOverTitle>
                </div>
                <SlideOverCloseButton />
              </SlideOverHeader>
              <div className="flex-1 overflow-y-auto px-5 py-4">
                {openGroup && (
                  <>
                    <div className="mb-4 grid grid-cols-3 gap-2 sm:grid-cols-6">
                      {DAY_LOG_STATUS_ORDER.map((status) => (
                        <Stat
                          key={status}
                          label={STATUS_LABELS[status]}
                          value={counterFor(openGroup.counters, status)}
                          highlight={status === "absent"}
                        />
                      ))}
                    </div>
                    <GroupDetail group={openGroup} />
                  </>
                )}
              </div>
              {openGroup ? (
                <SlideOverFooter className="flex-row flex-wrap justify-end gap-2">
                  {exportButtons(openGroup.group_id)}
                </SlideOverFooter>
              ) : null}
            </SlideOverContent>
          </SlideOver>
        </>
      }
    >
      {/* Inhaltskarte ohne eigenen Kopf: Titel, Datum und Exporte trägt die
          Kopfkarte darüber, hier stehen nur Zahlen und Gruppen. */}
      <SectionCard>
        {exportError && (
          <div className="mb-4">
            <Alert type="error" message={exportError} />
          </div>
        )}

        {data && (
          <>
            <div className="grid grid-cols-3 gap-2 sm:grid-cols-7">
              <Stat label="Kinder gesamt" value={data.counters.total} />
              {DAY_LOG_STATUS_ORDER.map((status) => (
                <Stat
                  key={status}
                  label={STATUS_LABELS[status]}
                  value={counterFor(data.counters, status)}
                  highlight={status === "absent"}
                />
              ))}
            </div>

            <div className="mt-4 grid gap-3 lg:grid-cols-2">
              {data.groups.map((group) => (
                <GroupCard
                  key={group.group_id}
                  group={group}
                  onOpen={() => setOpenGroupId(group.group_id)}
                />
              ))}
            </div>
          </>
        )}
      </SectionCard>
    </TenantPage>
  );
}
