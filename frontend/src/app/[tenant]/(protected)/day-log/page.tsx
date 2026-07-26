"use client";

// Tagesauswertung (#1456): per group and calendar day, every child with one
// day verdict — Anwesend / Krank / Klassenfahrt / Entschuldigt / Nicht
// eingeplant / Abwesend. Read-only evaluation of what NFC/web check-in and
// sick notes already record; gated by gdpr.attendance_log_enabled.
//
// Design follows the Anmeldungen/Planung surface language: one calm content
// section with an uppercase kicker, gray-50 stat blocks instead of colored
// chips, white per-group cards in a two-column grid, click opens the
// per-group detail modal with the status sections and group-scoped exports.

import { ArrowRight, Download, FileSpreadsheet, Printer } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Modal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
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
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "DayLogPage" });

const ERROR_MESSAGES: Record<DayLogErrorCode, string> = {
  feature_disabled:
    "Das Anwesenheitsprotokoll ist für diese Schule nicht aktiviert. Eine Administration kann es unter Einstellungen → Datenschutz einschalten.",
  not_group_supervisor:
    "Sie haben keinen Zugriff auf diese Gruppe. Sichtbar sind nur Gruppen, die Sie betreuen.",
  no_permitted_groups:
    "Ihrem Konto ist keine Gruppe zugeordnet. Die Tagesauswertung zeigt nur Gruppen, die Sie betreuen.",
  invalid_request:
    "Das gewählte Datum liegt außerhalb des sichtbaren Zeitraums.",
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
          highlight && value > 0 ? "text-[#CC2626]" : "text-gray-900"
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
    <article className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-gray-900">
            {group.name}
          </h3>
          <p className="mt-1 text-xs text-gray-500">
            {c.present} von {c.total} Kindern anwesend
          </p>
        </div>
        <button
          type="button"
          onClick={onOpen}
          className="inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          Details
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </button>
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
    </article>
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
              section.status === "absent" ? "text-[#CC2626]" : "text-gray-500"
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
  const [dateISO, setDateISO] = useState<string>(() => berlinTodayISO());
  const [data, setData] = useState<DayLogResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorCode, setErrorCode] = useState<DayLogErrorCode | null>(null);
  const [openGroupId, setOpenGroupId] = useState<string | null>(null);
  const [exporting, setExporting] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

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
  const minDate = useMemo(() => parseISODate(berlinTodayISO()), []);
  const maxDate = useMemo(() => parseISODate(berlinTodayISO()), []);

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
      // The tab must open synchronously inside the click gesture — popup
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
          // Popup blocked even in the gesture — last resort, may be blocked
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

  // Action trio in the Anmeldungen button idiom: Drucken as the dark primary,
  // the downloads as quiet white bordered actions. All sit on white surfaces.
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
      <Button
        type="button"
        variant="outline"
        size="md"
        className="gap-2 bg-white"
        disabled={!data || exporting !== null}
        onClick={() => void downloadExport("pdf", groupId)}
      >
        <Download className="h-4 w-4" aria-hidden />
        {exporting === `pdf-${groupId ?? "all"}` ? "Wird exportiert…" : "PDF"}
      </Button>
      <Button
        type="button"
        variant="outline"
        size="md"
        className="gap-2 bg-white"
        disabled={!data || exporting !== null}
        onClick={() => void downloadExport("xlsx", groupId)}
      >
        <FileSpreadsheet className="h-4 w-4" aria-hidden />
        {exporting === `xlsx-${groupId ?? "all"}`
          ? "Wird exportiert…"
          : "Excel"}
      </Button>
    </>
  );

  const selectedDate = parseISODate(dateISO);

  return (
    <div className="w-full">
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Tagesauswertung
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Anwesenheit pro Gruppe
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Wer war anwesend, krank, entschuldigt oder fehlt ohne Meldung –
              für heute.
            </p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
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
              className="w-full sm:w-44"
            />
            <div className="flex flex-wrap gap-2">{exportButtons()}</div>
          </div>
        </div>

        {exportError && (
          <div className="mt-4">
            <Alert type="error" message={exportError} />
          </div>
        )}

        {loading && (
          <div className="mt-4 grid gap-3 lg:grid-cols-2">
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-40 w-full" />
            <Skeleton className="h-40 w-full" />
          </div>
        )}

        {!loading && errorCode !== null && (
          <EmptyState
            className="mt-4"
            title={
              errorCode === "feature_disabled"
                ? "Anwesenheitsprotokoll deaktiviert"
                : "Tagesauswertung nicht verfügbar"
            }
            description={ERROR_MESSAGES[errorCode]}
          />
        )}

        {!loading && errorCode === null && data && (
          <>
            <div className="mt-4 grid grid-cols-3 gap-2 sm:grid-cols-7">
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

            {data.groups.length === 0 ? (
              <EmptyState
                className="mt-4"
                title="Keine Gruppen"
                description="Für diesen Tag sind keine Gruppen sichtbar."
              />
            ) : (
              <div className="mt-4 grid gap-3 lg:grid-cols-2">
                {data.groups.map((group) => (
                  <GroupCard
                    key={group.group_id}
                    group={group}
                    onOpen={() => setOpenGroupId(group.group_id)}
                  />
                ))}
              </div>
            )}
          </>
        )}
      </section>

      <Modal
        isOpen={openGroup !== null}
        onClose={() => setOpenGroupId(null)}
        title={openGroup ? `${openGroup.name} · ${formatDate(dateISO)}` : ""}
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-3xl"
        footer={
          openGroup ? (
            <div className="flex flex-wrap justify-end gap-2">
              {exportButtons(openGroup.group_id)}
            </div>
          ) : undefined
        }
      >
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
      </Modal>
    </div>
  );
}
