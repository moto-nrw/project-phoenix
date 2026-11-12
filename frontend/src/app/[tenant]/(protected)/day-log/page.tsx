"use client";

// Tagesauswertung (#1456): per group and calendar day, every child with one
// day verdict: Anwesend / Krank / Klassenfahrt / Entschuldigt / Nicht
// eingeplant / Abwesend. Read-only evaluation of what NFC/web check-in and
// sick notes already record; gated by gdpr.attendance_log_enabled.
//
// Design follows the Anmeldungen/Planung surface language: one calm content
// section, gray-50 stat blocks instead of colored
// chips, white per-group cards in a two-column grid, click opens the
// per-group detail modal with the status sections and group-scoped exports.

import { SectionLabel } from "~/components/ui/section-label";
import { FramedList } from "~/components/ui/framed-list";
import { ListRowLink } from "~/components/ui/list-row-link";
import {
  ArrowRight,
  Download,
  FileSpreadsheet,
  FileText,
  Printer,
} from "lucide-react";
import { StatCard } from "~/components/ui/stat-card";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuEntry } from "~/components/ui/page-header/OverflowMenu";
import {
  SlideOver,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import {
  apiErrorFromResponse,
  ApiError,
  transportFetch,
  wireErrorCode,
} from "~/lib/api-error";
import {
  DAY_LOG_STATUS_COLORS,
  DAY_LOG_STATUS_ORDER,
  dayLogExportUrl,
  dayLogSourceLabel,
  fetchDayLog,
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
import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "DayLogPage" });

/**
 * Ausgeschaltete Funktion und Konto ohne sichtbare Gruppe sind Zustände, kein
 * Fehler: sie stehen als Leerzustand mit dem nächsten Schritt. Erkannt am
 * Code, nie am Text (#2517).
 */
type DayLogState = "disabled" | "no_groups";

const STATE_EMPTY: Record<DayLogState, { title: string; description: string }> =
  {
    disabled: {
      title: "Anwesenheitsprotokoll ist ausgeschaltet",
      description:
        "Ihre Leitung kann es in den Einstellungen unter Datenschutz einschalten.",
    },
    no_groups: {
      title: "Noch keine Gruppen für Ihr Konto",
      description:
        "Für Ihr Konto ist keine Gruppe sichtbar. Bitte wenden Sie sich an Ihre Leitung.",
    },
  };

function dayLogState(error: unknown): DayLogState | null {
  const code = error instanceof ApiError ? wireErrorCode(error.code) : null;
  if (code === "students.day_log_disabled") return "disabled";
  if (code === "students.day_log_no_groups") return "no_groups";
  return null;
}

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

type ExportFormat = "pdf" | "docx" | "xlsx";

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
    // Ohne Rückkehr endet der Tag mit dem Weg zum Unterricht, nicht mit „gegangen“.
    const leftForClass = student.class_periods?.some((p) => !p.until);
    const departure = leftForClass ? "zum Unterricht" : "gegangen";
    const parts = [
      `Ankunft ${from} · ${until ? `${departure} ${until}` : "noch anwesend"}`,
    ];
    for (const period of student.class_periods ?? []) {
      if (period.until) {
        parts.push(
          `im Unterricht ${formatClock(period.from)} bis ${formatClock(period.until)} Uhr`,
        );
      }
    }
    if (student.check_out_note) {
      parts.push(`Grund für das frühe Gehen: ${student.check_out_note}`);
    }
    if (student.hint) parts.push(student.hint);
    return parts.join(" · ");
  }
  const parts: string[] = [];
  if (student.reported_at) {
    parts.push(`gemeldet ${formatClock(student.reported_at)} Uhr`);
  }
  const source = dayLogSourceLabel(student.source);
  if (source) parts.push(source);
  return parts.join(" · ");
}

// Nur „Ohne Meldung“ färbt, und nur, wenn jemand nachfassen muss.
function counterTone(
  status: (typeof DAY_LOG_STATUS_ORDER)[number],
  value: number,
) {
  return status === "absent" && value > 0 ? "red" : "neutral";
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
          variant="surface"
          size="md"
          onClick={onOpen}
          className="shrink-0 gap-2"
          tooltip="Zeigt die Kinder dieser Gruppe nach Status geordnet."
        >
          Details
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
      <div className="mt-4 grid grid-cols-3 gap-2">
        {DAY_LOG_STATUS_ORDER.map((status) => (
          <StatCard
            variant="tile"
            key={status}
            label={STATUS_LABELS[status]}
            value={counterFor(c, status)}
            tone={counterTone(status, counterFor(c, status))}
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
      <ListRowLink
        href={tenantPath(`/students/${student.student_id}?from=/day-log`)}
        title={`${student.last_name}, ${student.first_name}`}
        subtitle={`${student.school_class}${detail ? ` · ${detail}` : ""}`}
        badge={
          <StatusColorBadge
            label={student.label}
            color={DAY_LOG_STATUS_COLORS[student.status]}
          />
        }
      />
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
          <SectionLabel className="mb-1">
            {MODAL_SECTION_TITLES[section.status]} ({section.students.length})
          </SectionLabel>
          <FramedList>
            {section.students.map((student) => (
              <StudentRow key={student.student_id} student={student} />
            ))}
          </FramedList>
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
  const [state, setState] = useState<DayLogState | null>(null);
  // Ladefehler stehen im Gerüst, mit Wiederholen (#2517).
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const [reloadKey, setReloadKey] = useState(0);
  // Bis der Katalogtext da ist, bleibt das Skelett stehen.
  const [failed, setFailed] = useState(false);
  const [openGroupId, setOpenGroupId] = useState<string | null>(null);
  const [exporting, setExporting] = useState<string | null>(null);
  // Export und Drucken sind Aktionen ohne Formular: Fehler als Toast.
  const { show: showActionError } = useApiErrorDisplay();
  const retryExportRef = useRef<() => void>(() => undefined);

  // Keep an open page on the Berlin calendar day. The backend only accepts
  // today's roster, so a tab spanning midnight must refetch rather than retain
  // yesterday's now-invalid date.
  useEffect(() => {
    setDateISO((selected) => (selected === todayISO ? selected : todayISO));
  }, [todayISO]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setState(null);
    setFailed(false);
    clearLoadError();
    fetchDayLog(dateISO)
      .then((response) => {
        if (!cancelled) setData(response);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setData(null);
        const pageState = dayLogState(error);
        if (pageState) {
          setState(pageState);
          return;
        }
        logger.error("day_log_fetch_failed", {
          date: dateISO,
          error: error instanceof Error ? error.message : String(error),
        });
        setFailed(true);
        void showLoadError(error, {
          object: "die Tagesauswertung",
          retry: () => setReloadKey((key) => key + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [dateISO, reloadKey, showLoadError, clearLoadError]);

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
      try {
        const res = await transportFetch(
          dayLogExportUrl(dateISO, format, groupId),
        );
        if (!res.ok) {
          throw await apiErrorFromResponse(res, "day log export failed");
        }
        downloadBlob(
          await res.blob(),
          filenameFromDisposition(res) ??
            `tagesauswertung-${dateISO}.${format}`,
        );
      } catch (error) {
        logger.error("day_log_export_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        retryExportRef.current = () => void downloadExport(format, groupId);
        void showActionError(error, {
          object: "die Exportdatei",
          retry: () => retryExportRef.current(),
        });
      } finally {
        setExporting(null);
      }
    },
    [dateISO, showActionError],
  );

  // Drucken: the same PDF in a new tab with the browser's print dialog, like
  // every other print action (Notfallliste, Tageslisten, Pläne).
  const printPdf = useCallback(
    async (groupId?: string) => {
      // The tab must open synchronously inside the click gesture; popup
      // blockers discard windows opened after an await. Close it again if
      // the export fails.
      const tab = window.open("", "_blank");
      if (tab) tab.opener = null;
      setExporting(`print-${groupId ?? "all"}`);
      try {
        const res = await transportFetch(
          dayLogExportUrl(dateISO, "pdf", groupId),
        );
        if (!res.ok) {
          throw await apiErrorFromResponse(res, "day log print failed");
        }
        // Without a tab (popup blocked even in the gesture) the helper makes
        // a last attempt itself and throws if that is blocked too.
        await openBlobForPrint(await res.blob(), tab);
      } catch (error) {
        tab?.close();
        logger.error("day_log_print_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        retryExportRef.current = () => void printPdf(groupId);
        void showActionError(error, {
          object: "die Druckansicht",
          retry: () => retryExportRef.current(),
        });
      } finally {
        setExporting(null);
      }
    },
    [dateISO, showActionError],
  );

  // Eine sichtbare Aktion neben dem Titel, alles Weitere im Menü: Drucken ist
  // der tägliche Griff, PDF und Excel sind der seltene. Vorher standen alle
  // drei nebeneinander und füllten die halbe Kopfzeile -- dieselbe Reihe, die
  // die Statistik hatte.
  // Was ein Export enthält: der ganze Tag oder die offene Gruppe.
  const exportScope = (groupId?: string) =>
    groupId ? "die Auswertung dieser Gruppe" : "die Tagesauswertung";
  const exportActions = (groupId?: string): OverflowMenuEntry[] => [
    { kind: "header", label: "Herunterladen" },
    {
      label:
        exporting === `pdf-${groupId ?? "all"}` ? "Wird exportiert…" : "PDF",
      icon: <Download className="h-4 w-4" aria-hidden />,
      description: `Speichert ${exportScope(groupId)} als PDF auf Ihrem Gerät.`,
      onClick: () => void downloadExport("pdf", groupId),
      disabled: !data || exporting !== null,
    },
    {
      label:
        exporting === `docx-${groupId ?? "all"}` ? "Wird exportiert…" : "Word",
      icon: <FileText className="h-4 w-4" aria-hidden />,
      description: `Speichert ${exportScope(groupId)} als Word-Datei auf Ihrem Gerät.`,
      onClick: () => void downloadExport("docx", groupId),
      disabled: !data || exporting !== null,
    },
    {
      label:
        exporting === `xlsx-${groupId ?? "all"}` ? "Wird exportiert…" : "Excel",
      icon: <FileSpreadsheet className="h-4 w-4" aria-hidden />,
      description: `Speichert ${exportScope(groupId)} als Excel-Datei auf Ihrem Gerät.`,
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
        tooltip={`Öffnet ${exportScope(groupId)} als PDF zum Drucken.`}
      >
        <Printer className="h-4 w-4" aria-hidden />
        {exporting === `print-${groupId ?? "all"}`
          ? "Wird geöffnet…"
          : "Drucken"}
      </Button>
      <OverflowMenu
        items={exportActions(groupId)}
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
      loading={loading || (failed && load.error === null)}
      // Ein Ladefehler ist `error`. Eine ausgeschaltete Funktion oder ein
      // fehlender Personaleintrag ist dagegen ein Zustand, kein Fehler: er
      // steht als Leerzustand mit dem nächsten Schritt.
      error={load.error}
      empty={
        state !== null
          ? STATE_EMPTY[state]
          : load.error === null && data !== null && data.groups.length === 0
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
          {/* oxlint-disable-next-line ui-kit/slide-over-pending -- zeigt die Gruppe und lädt Export oder Druck herunter; schreibt nichts. */}
          <SlideOver
            open={openGroup !== null}
            onOpenChange={(open) => {
              if (!open) setOpenGroupId(null);
            }}
          >
            <SlideOverContent size="lg">
              <SlideOverHeader>
                <SlideOverTitle>
                  {openGroup
                    ? `${openGroup.name} · ${formatDate(dateISO)}`
                    : ""}
                </SlideOverTitle>
              </SlideOverHeader>
              <div className="flex-1 overflow-y-auto px-5 py-4">
                {openGroup && (
                  <>
                    <div className="mb-4 grid grid-cols-3 gap-2 sm:grid-cols-6">
                      {DAY_LOG_STATUS_ORDER.map((status) => (
                        <StatCard
                          variant="tile"
                          key={status}
                          label={STATUS_LABELS[status]}
                          value={counterFor(openGroup.counters, status)}
                          tone={counterTone(
                            status,
                            counterFor(openGroup.counters, status),
                          )}
                        />
                      ))}
                    </div>
                    <GroupDetail group={openGroup} />
                  </>
                )}
              </div>
              {openGroup ? (
                <SlideOverFooter>
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
        {data && (
          <>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4 2xl:grid-cols-7">
              <StatCard
                label="Kinder gesamt"
                value={data.counters.total}
                tone="neutral"
              />
              {DAY_LOG_STATUS_ORDER.map((status) => (
                <StatCard
                  key={status}
                  label={STATUS_LABELS[status]}
                  value={counterFor(data.counters, status)}
                  tone={counterTone(status, counterFor(data.counters, status))}
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
