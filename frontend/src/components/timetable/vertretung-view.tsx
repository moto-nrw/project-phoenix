"use client";

/**
 * Vertretung (#1886/Planung-Redesign, docs/planung-redesign/docs/07-vertretung.md).
 *
 * Der heute-zentrierte Zweiteiler: links die Störungsliste des Tages
 * (VertretungDayList), rechts der Tag als einspaltige Kalenderansicht
 * (WeeklyCalendarGrid). Der Editor bleibt der bestehende
 * SubstitutionSlideOver; alle Schreibvorgänge laufen unverändert über den
 * atomaren Deviations-Pfad. Es gibt keine eigene Vertretungs-Entität.
 *
 * Zusätzlich gibt es die Wochenansicht (#2030): dieselbe Fläche, nur über
 * Mo–Fr — links die nach Tagen gruppierte Störungsliste
 * (VertretungWeekList), rechts dasselbe Raster mit fünf Tagesspalten. Sie ist
 * eine reine Anzeigevariante: dieselben Daten (die Woche wird ohnehin schon
 * geladen), derselbe Editor, dieselbe Störungsdefinition. Die Tagesansicht
 * bleibt der Standard beim Öffnen der Seite.
 *
 * URL-Vokabular: höchstens `d`, `view`, `block`, `verlauf`. `d` ist der
 * angezeigte Berlin-Kalendertag (ohne `d` gilt heute; Wochenendtage werden auf
 * den folgenden Montag normalisiert, die Leiste zeigt nur Mo–Fr) und ankert in
 * der Wochenansicht die gezeigte Woche, `view=woche` schaltet auf die
 * Wochenansicht (jeder andere Wert, auch ein fehlender, ist die Tagesansicht),
 * `block` öffnet den Editor für die materialisierte Instanz mit dieser ID,
 * `verlauf=1` schaltet den Editor auf den Verlaufs-Reiter. Der Filterzustand
 * "Nur Störungen | Ganzer Tag" ist lokaler State, kein URL-Parameter.
 *
 * Das Ladeverhalten übernimmt die Orchestrierung der früheren
 * vertretungsplan-view.tsx als Verhaltensvertrag (Abschnitt 2.4): Woche laden,
 * Gaps mit Heute-Klemmung, Personalliste strict, Settings-Gate. Die
 * kommentierten Invarianten (Fehler nie als leerer Plan rendern, `revalidate`
 * schluckt eigene Fehler und meldet ein committetes Save nie als Fehler,
 * Toast-Deduplizierung über die Fehlermeldung, `gapsUnavailable`-Platzhalter
 * statt erfundener Null) überleben wörtlich.
 */

import { CalendarRange } from "lucide-react";
import { useSession } from "next-auth/react";
import { Suspense, useCallback, useEffect, useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { Button } from "~/components/ui/button";
import { CoverageIndicator } from "~/components/ui/coverage-indicator";
import {
  PlanningContextBar,
  PlanningDayChip,
} from "~/components/ui/planning-context-bar";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { BulkSubstitutionModal } from "~/components/timetable/bulk-substitution-modal";
import { SubstitutionSlideOver } from "~/components/timetable/substitution-slide-over";
import { cancelledToast } from "~/components/timetable/guardian-notice-toast";
import { VertretungCoverageNotice } from "~/components/timetable/vertretung-coverage-notice";
import {
  VertretungDayList,
  type VertretungDayListMode,
} from "~/components/timetable/vertretung-day-list";
import { VertretungContentSkeleton } from "~/components/timetable/vertretung-skeleton";
import { VertretungWeekList } from "~/components/timetable/vertretung-week-list";
import { timetableSurface } from "~/components/timetable/timetable-style";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import {
  berlinTodayISO,
  isValidISODate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { useTimetableDayHours } from "~/lib/hooks/use-timetable-day-hours";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { useUrlParams } from "~/lib/hooks/use-url-params";
import { createLogger } from "~/lib/logger";
import { getSettingValue } from "~/lib/settings-api";
import { staffShiftService } from "~/lib/shift-api";
import { staffService } from "~/lib/staff-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { timetableService } from "~/lib/timetable-api";
import {
  VERTRETUNG_GAPS_KEY_PREFIX,
  VERTRETUNG_WEEK_KEY_PREFIX,
  formatWeekLabel,
  getGermanWeekdayShort,
  getWeekRange,
  getWeekdays,
  nextWorkdayISO,
} from "~/lib/timetable-helpers";
import type {
  ApplyDeviationsInput,
  ApplyDeviationsResponse,
} from "~/lib/timetable-types";

const logger = createLogger({ component: "Vertretung" });
const HOUR_HEIGHT_PX = 90;
const VERTRETUNG_STAFF_LIST_KEY = "vertretung-staff-list";
// Das verbindliche URL-Vokabular. updateUrlParams baut die URL aus dieser
// Allowlist neu auf, damit fremde Params (?utm_source=…) nicht jeden
// Tageswechsel überleben. Muss mit ALLOWED_PARAMS in
// e2e/vertretung-flow.spec.ts übereinstimmen.
const ALLOWED_URL_PARAMS = ["d", "view", "block", "verlauf"] as const;

/** Anzeigevariante der Fläche. Standard beim Öffnen ist immer "tag". */
type VertretungViewMode = "tag" | "woche";

function shiftDayISO(iso: string, delta: number): string {
  const d = parseISODate(iso);
  d.setDate(d.getDate() + delta);
  return toISODate(d);
}

/** "14.07." — Tagesdatum für einen Wochenleisten-Chip. */
function dayChipLabel(d: Date): string {
  return `${String(d.getDate()).padStart(2, "0")}.${String(
    d.getMonth() + 1,
  ).padStart(2, "0")}.`;
}

function deviationSuccessMessage(
  input: ApplyDeviationsInput,
  result: ApplyDeviationsResponse,
  staffNames: ReadonlyMap<string, string>,
): string {
  const appointmentCount = new Set(
    result.affectedInstances.map((affected) => affected.instanceId),
  ).size;
  if (appointmentCount === 0) return "Vertretung aktualisiert";

  const staffIds = new Set<string>();
  input.absences?.forEach((absence) => staffIds.add(absence.staffId));
  input.substitutions?.forEach((substitution) =>
    staffIds.add(substitution.absentStaffId),
  );
  input.presences?.forEach((presence) => staffIds.add(presence.staffId));
  input.substitutionRemovals?.forEach((removal) =>
    staffIds.add(removal.staffId),
  );

  const appointmentText =
    appointmentCount === 1
      ? "1 Termin wurde"
      : `${appointmentCount} Termine wurden`;
  if (staffIds.size === 1) {
    const staffId = [...staffIds][0]!;
    const name = staffNames.get(staffId);
    if (name) return `${appointmentText} für ${name} angepasst.`;
  }
  if (staffIds.size > 1) {
    return `${appointmentText} für ${staffIds.size} Personen angepasst.`;
  }
  return `${appointmentText} angepasst.`;
}

function VertretungContent() {
  const { data: session, status } = useSession();
  const { params, updateParams: updateUrlParams } =
    useUrlParams(ALLOWED_URL_PARAMS);
  const toast = useToast();
  const tenantMutate = useTenantMutate();
  const { dayStartHour, dayEndHour } = useTimetableDayHours();

  // Lesen braucht `schedules:read` (die Listen-Endpunkte), jeder Save braucht
  // `schedules:manage`. Ohne canManage werden alle Editier-Kontrollen
  // ausgeblendet, Liste und Verlauf bleiben lesbar (Abschnitt 1/9).
  const canManageSchedules = hasPermission(session, "schedules:manage");
  // Der Dienstplan-Hinweis über der Liste liest die Dienstplan-Übersicht. Deren
  // Endpunkt verlangt mehr Rechte als die Vertretung selbst
  // (Administrator + time_tracking:manage + schedules:read + users:read,
  // siehe use-dienstplan-data.ts). Der Dienstplan leitet Nicht-Admins nach
  // /staff um; ohne alle Voraussetzungen entfällt der Hinweis daher still,
  // statt einen 403 oder einen toten Link zu erzeugen.
  const canReadCoverage =
    isAdmin(session) &&
    hasPermission(session, "time_tracking:manage") &&
    hasPermission(session, "schedules:read") &&
    hasPermission(session, "users:read");
  const tenantPath = useTenantAwarePath();

  // URL-Vokabular: d / block / verlauf — sonst nichts (Abschnitt 1).
  // Wochenendtage (Erstaufruf am Sa/So oder ?d=-Deeplink) snappen auf den
  // folgenden Montag — der eine Guard am Lese-Ort deckt alle Pfade ab, weil
  // dayISO nach jedem replaceState hier re-derived wird. Ein ungültiges `d`
  // (?d=foo oder ?d=2026-02-31) fällt auf heute zurück, statt NaN-Datumsketten
  // bzw. einen stillen Monats-Überlauf in die Fetch-Fenster zu speisen.
  const rawDay = params.d;
  const dayISO = nextWorkdayISO(
    rawDay !== null && isValidISODate(rawDay) ? rawDay : berlinTodayISO(),
  );
  const selectedInstanceId = params.block;
  const historyOpen = params.verlauf === "1";
  // Ansichtsvariante aus der URL. Nur "woche" schaltet um; jeder andere Wert
  // (auch ein fehlender oder ein Tippfehler) ist die Tagesansicht, damit ein
  // kaputter Deep-Link nie auf einer unerwarteten Fläche landet.
  const view: VertretungViewMode = params.view === "woche" ? "woche" : "tag";
  const isWeekView = view === "woche";

  const today = berlinTodayISO();
  // Ziel des Heute-Buttons: am Wochenende der nächste Montag. `today` selbst
  // bleibt überall sonst der echte Kalendertag (isPast, Gaps-Klemmung).
  const todayTarget = nextWorkdayISO(today);

  // Woche, die `d` enthält (Fetch-Fenster bleibt Mo-So, die Leiste zeigt nur
  // Mo-Fr). Immer über den Montag ankern, nie getWeekdays(parseISODate(d))
  // direkt: getWeekdays validiert nicht, dass sein Argument ein Montag ist.
  const range = useMemo(() => getWeekRange(parseISODate(dayISO), 0), [dayISO]);
  const fromISO = toISODate(range.from);
  const toISO = toISODate(range.to);
  const weekDays = useMemo(
    () => getWeekdays(range.from).slice(0, 5),
    [range.from],
  );
  // Label über Mo–Fr, nicht über das Mo–So-Fetch-Fenster: die Ansicht zeigt
  // fünf Spalten, ein bis Sonntag laufendes Label würde zwei Tage versprechen,
  // die es nicht gibt.
  const weekLabel = useMemo(
    () =>
      formatWeekLabel(range.from, weekDays[weekDays.length - 1] ?? range.to),
    [range.from, range.to, weekDays],
  );
  // Das Fetch-Fenster ist Mo–So, angezeigt wird nur Mo–Fr. Alles, was die
  // Wochenansicht zeigt oder zählt, wird daher auf die Schultage begrenzt —
  // sonst zählte ein (in einer OGS seltener) Wochenendtermin in einen Zähler,
  // dessen Spalte es gar nicht gibt.
  const weekDayISOSet = useMemo(
    () => new Set(weekDays.map(toISODate)),
    [weekDays],
  );

  const weekSwrKey = `${VERTRETUNG_WEEK_KEY_PREFIX}${fromISO}-${toISO}`;
  // Die Lücken-Erkennung ist vorwärtsgerichtet: der Endpunkt lehnt ein
  // vergangenes `date` ab, also den Fensterstart auf heute klemmen und für
  // vollständig vergangene Wochen den Abruf ganz überspringen.
  const gapsFromISO = fromISO < today ? today : fromISO;
  const loadGaps = toISO >= today;
  const gapsSwrKey = `${VERTRETUNG_GAPS_KEY_PREFIX}${gapsFromISO}-${toISO}`;

  // Fenster des Dienstplan-Abgleichs: Mo–Fr, identisch zum Wochenraster des
  // Dienstplans, damit beide Flächen denselben SWR-Cache benutzen (der Key ist
  // bewusst wörtlich derselbe wie in use-dienstplan-data.ts). Eine vollständig
  // vergangene Woche braucht keinen Abgleich.
  const coverageFromISO = toISODate(weekDays[0] ?? range.from);
  const coverageToISO = toISODate(
    weekDays[weekDays.length - 1] ?? weekDays[0] ?? range.to,
  );
  const loadCoverage = canReadCoverage && coverageToISO >= today;
  const coverageSwrKey = `dienstplan-overview-${coverageFromISO}-${coverageToISO}`;

  const { data, isLoading, error } = useSWRAuth(
    status === "authenticated" ? weekSwrKey : null,
    () => timetableService.getWeek(fromISO, toISO),
  );
  const { data: gapsData, error: gapsError } = useSWRAuth(
    status === "authenticated" && loadGaps ? gapsSwrKey : null,
    () => timetableService.getGaps(gapsFromISO, toISO),
  );
  const { data: staffData, error: staffError } = useSWRAuth(
    status === "authenticated" ? VERTRETUNG_STAFF_LIST_KEY : null,
    // strict: ein Backend-Fehler muss rejecten (nicht zu [] resolven), damit
    // staffError feuert und der Picker den Fehler statt "kein Personal" zeigt.
    () => staffService.getAllStaff(undefined, { strict: true }),
  );
  // Dienstplan-Abgleich für den Hinweis über der Liste. Bewusst NICHT strict
  // und ohne Toast: der Hinweis ist eine Zusatzinformation, sein Ausfall darf
  // die Vertretungsarbeit nicht kommentieren (er verschwindet dann einfach).
  const { data: coverageData, error: coverageError } = useSWRAuth(
    status === "authenticated" && loadCoverage ? coverageSwrKey : null,
    () => staffShiftService.getOverview(coverageFromISO, coverageToISO),
  );

  // Route-Gate wie in der alten View: Settings-Schema -> timetable.enabled.
  // fetchSettingsSchema liefert null, wenn der Nutzer keine Settings lesen darf;
  // die Seite rendert dann normal (bewusster Default, wie /timetables).
  const { data: settingsSchema, isLoading: settingsSchemaLoading } =
    useSettingsSchema(status === "authenticated", {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    });
  const timetableDisabled =
    getSettingValue(settingsSchema, "timetable.enabled") === false;

  // Fetch-Fehler erhalten, damit "Keine Termine"/Null-Lücken nie so gerendert
  // werden, als hätte ein fehlgeschlagener Request Erfolg gehabt. SWR-Retries
  // erzeugen pro Versuch einen frischen Error, daher die Toasts über die
  // Fehlermeldung deduplizieren.
  const weekErrorMessage = error
    ? error instanceof Error
      ? error.message
      : String(error)
    : null;
  const gapsErrorMessage = gapsError
    ? gapsError instanceof Error
      ? gapsError.message
      : String(gapsError)
    : null;
  // Ein fehlgeschlagener Personalabruf darf nie wie eine leere Ersatzliste
  // aussehen — jeder Picker würde stumm deaktivieren und Namen fielen auf IDs
  // zurück, ununterscheidbar von einer Schule ohne Personal.
  const staffErrorMessage = staffError
    ? staffError instanceof Error
      ? staffError.message
      : String(staffError)
    : null;

  useEffect(() => {
    if (!weekErrorMessage) return;
    logger.error("week_load_failed", { error: weekErrorMessage });
    toast.error(`Vertretung konnte nicht geladen werden: ${weekErrorMessage}`);
  }, [weekErrorMessage, toast]);

  useEffect(() => {
    if (!gapsErrorMessage) return;
    logger.error("gaps_load_failed", { error: gapsErrorMessage });
    toast.error(
      `Offene Lücken konnten nicht geprüft werden: ${gapsErrorMessage}`,
    );
  }, [gapsErrorMessage, toast]);

  useEffect(() => {
    if (!staffErrorMessage) return;
    logger.error("staff_load_failed", { error: staffErrorMessage });
    toast.error(
      `Personalliste konnte nicht geladen werden: ${staffErrorMessage}`,
    );
  }, [staffErrorMessage, toast]);

  // Nur protokollieren, kein Toast: siehe Kommentar am Abruf.
  useEffect(() => {
    if (!coverageError) return;
    logger.warn("coverage_notice_load_failed", {
      error:
        coverageError instanceof Error
          ? coverageError.message
          : String(coverageError),
    });
  }, [coverageError]);

  const instances = useMemo(() => data?.instances ?? [], [data?.instances]);
  const weekdayInstances = useMemo(
    () => instances.filter((inst) => weekDayISOSet.has(inst.date)),
    [instances, weekDayISOSet],
  );
  const dayInstances = useMemo(
    () => instances.filter((inst) => inst.date === dayISO),
    [instances, dayISO],
  );
  const staffOptions = useMemo(
    () => (staffData ?? []).map((s) => ({ id: s.id, name: s.name })),
    [staffData],
  );
  const staffNames = useMemo(
    () => new Map((staffData ?? []).map((s) => [s.id, s.name])),
    [staffData],
  );
  const selectedInstance = useMemo(
    () => instances.find((inst) => inst.id === selectedInstanceId) ?? null,
    [instances, selectedInstanceId],
  );
  const editorDayInstances = useMemo(
    () =>
      selectedInstance
        ? instances.filter((inst) => inst.date === selectedInstance.date)
        : dayInstances,
    [dayInstances, instances, selectedInstance],
  );

  // Wochenweite Lücken (das geladene Fenster) — Basis für die Wochenansicht
  // und für die Tagesfilterung darunter.
  const weekGaps = useMemo(
    () => (gapsData?.gaps ?? []).filter((g) => weekDayISOSet.has(g.date)),
    [gapsData?.gaps, weekDayISOSet],
  );
  const weekAcknowledged = useMemo(
    () =>
      (gapsData?.acknowledged ?? []).filter((g) => weekDayISOSet.has(g.date)),
    [gapsData?.acknowledged, weekDayISOSet],
  );
  const weekGapInstanceIds = useMemo(
    () => new Set(weekGaps.map((g) => g.instanceId)),
    [weekGaps],
  );

  // Tagesgefilterte Lücken für Liste und Zähler.
  const dayGaps = useMemo(
    () => weekGaps.filter((g) => g.date === dayISO),
    [weekGaps, dayISO],
  );
  const dayAcknowledged = useMemo(
    () => weekAcknowledged.filter((g) => g.date === dayISO),
    [weekAcknowledged, dayISO],
  );
  // Instanz-IDs der offenen (NICHT quittierten) Lücken des Tages für die
  // Block-Markierung im Kalender. Quittierte liegen in `acknowledged` und
  // bekommen kein Lücken-Icon (sie sind bewusst unbesetzt).
  const dayGapInstanceIds = useMemo(
    () => new Set(dayGaps.map((g) => g.instanceId)),
    [dayGaps],
  );
  // Pro-Tag-Zahlen der Wochenleiste durch Client-Gruppierung der `gaps`.
  const gapsByDate = useMemo(() => {
    const m = new Map<string, number>();
    for (const g of weekGaps) {
      m.set(g.date, (m.get(g.date) ?? 0) + 1);
    }
    return m;
  }, [weekGaps]);

  // Der Bereich ist immer tageszentriert. Ein vergangener Tag (auch innerhalb
  // der laufenden Woche, wo loadGaps true bleibt, der Fensterstart aber auf
  // heute geklemmt ist) hat keine geladenen Lücken — Zähler zeigen einen
  // Strichplatzhalter statt einer erfundenen Null; ebenso bei Fehler oder
  // solange die Lücken noch laden.
  // Eine gemeinsame Basis für Zähler UND Chips, damit ein künftiger weiterer
  // "nicht verfügbar"-Grund nicht nur in einem der beiden Ausdrücke landet.
  const gapsLoaded =
    loadGaps && gapsErrorMessage === null && gapsData !== undefined;
  const gapsUnavailable = !gapsLoaded || dayISO < today;
  // Erster Tag der Woche mit Lückendaten (das Fenster ist auf heute geklemmt);
  // null, solange gar nichts geladen ist. Die Wochenliste leitet daraus pro Tag
  // ab, ob sie eine Störungslage behaupten darf.
  const hasVisibleWeekdayWithGaps = weekDays.some(
    (day) => toISODate(day) >= gapsFromISO,
  );
  const gapsAvailableFrom =
    gapsLoaded && hasVisibleWeekdayWithGaps ? gapsFromISO : null;

  // Zähler der Kontextleiste: die Tagesansicht zählt den Tag, die
  // Wochenansicht die geladene Woche. Ein auf heute geklemmtes Fenster ist
  // keine vollständige Wochenaussage — das steht im Tooltip, statt eine
  // vollständige Zahl vorzutäuschen.
  const countsUnavailable = isWeekView
    ? !gapsLoaded || !hasVisibleWeekdayWithGaps
    : gapsUnavailable;
  const countsClamped = isWeekView && gapsLoaded && fromISO < today;
  const openCount = isWeekView ? weekGaps.length : dayGaps.length;
  const ackCount = isWeekView
    ? weekAcknowledged.length
    : dayAcknowledged.length;
  const countScope = isWeekView
    ? countsClamped
      ? "ab heute in dieser Woche"
      : "in dieser Woche"
    : "an diesem Tag";

  // Einsätze, die der Dienstplan nicht abdeckt (Person ist eingeteilt, hat aber
  // keine passende Schicht). KEIN Störungsfall: die Störungsdefinition bleibt
  // vierteilig, deshalb zählt das hier nirgends in openCount/ackCount oder die
  // Tageschips, sondern ausschließlich in den eigenen Hinweis. Vergangene Tage
  // bleiben still, genau wie bei den Lücken; eine Woche ohne gepflegten
  // Dienstplan liefert dienstplanInUse = false und damit gar nichts.
  const uncoveredCount = useMemo(() => {
    if (coverageError || !coverageData?.dienstplanInUse) return 0;
    const inScope = (date: string) =>
      date >= today && (isWeekView ? weekDayISOSet.has(date) : date === dayISO);
    return coverageData.assignments.filter(
      (assignment) =>
        assignment.coverageStatus === "uncovered" && inScope(assignment.date),
    ).length;
  }, [coverageData, coverageError, dayISO, isWeekView, today, weekDayISOSet]);
  // Der Dienstplan versteht dasselbe `d` wie diese Ansicht; in der
  // Wochenansicht landet man auf dem Montag der gezeigten Woche.
  const dienstplanHref = tenantPath(
    `/dienstplan?d=${isWeekView ? coverageFromISO : dayISO}`,
  );

  // Filterzustand ist LOKAL, kein URL-Parameter (verbindliches
  // Drei-Parameter-Vokabular). Standard "Nur Störungen" (aus K2).
  const [mode, setMode] = useState<VertretungDayListMode>("stoerungen");

  // Sammel-Vertretung (#2284): Modalzustand ist lokal, kein URL-Parameter —
  // das URL-Vokabular bleibt bei d/view/block/verlauf.
  const [bulkOpen, setBulkOpen] = useState(false);

  const goToDay = useCallback(
    (iso: string) => updateUrlParams({ d: iso, block: null, verlauf: null }),
    [updateUrlParams],
  );
  // Ansichtswechsel schreibt nur `view` (die Tagesansicht ist die
  // Param-Entfernung, damit sie der Standard bleibt). Datum und geöffneter
  // Editor bleiben stehen: Woche und Tag sind zwei Sichten auf dieselben
  // bereits geladenen Daten, kein Kontextwechsel.
  const setView = useCallback(
    (next: VertretungViewMode) =>
      updateUrlParams({ view: next === "woche" ? "woche" : null }),
    [updateUrlParams],
  );
  // Klick auf eine Tageskopfzeile der Wochenliste: in die Tagesansicht dieses
  // Tages wechseln (der Weg von der Übersicht in die Bearbeitung).
  const openDayView = useCallback(
    (iso: string) =>
      updateUrlParams({ d: iso, view: null, block: null, verlauf: null }),
    [updateUrlParams],
  );
  // `verlauf` wird bei jeder Blockauswahl explizit abgeräumt (Muster der alten
  // handleSelectInstance): sonst klebt ein früher geöffneter Verlaufs-Reiter an
  // der URL und der nächste per Klick geöffnete Block startet im Verlauf statt
  // im Formular. Deep-Links mit verlauf=1 laufen nicht über diesen Pfad.
  const openEditor = useCallback(
    (instanceId: string) =>
      updateUrlParams({ block: instanceId, verlauf: null }),
    [updateUrlParams],
  );
  const closeEditor = useCallback(
    () => updateUrlParams({ block: null, verlauf: null }),
    [updateUrlParams],
  );

  // Cache-Refresh ist bewusst GETRENNT von der committeten Mutation: SWRs
  // mutate rejected standardmäßig, wenn eine Revalidierung fehlschlägt, und das
  // darf ein bereits committetes Save nie in einen gemeldeten Fehler
  // verwandeln. revalidate schluckt daher eigene Fehler und meldet stattdessen
  // einen eigenen "bitte neu laden"-Toast.
  const revalidate = useCallback(async () => {
    try {
      // Der Dienstplan-Abgleich hängt an den konkreten Personalzeilen, ein
      // Vertretungs-Save kann ihn also verändern (Ersatz kommt hinzu, geplante
      // Person fällt weg). Er wird deshalb mitrevalidiert, wenn er überhaupt
      // aktiv ist.
      await Promise.all([
        tenantMutate(weekSwrKey),
        tenantMutate(gapsSwrKey),
        ...(loadCoverage ? [tenantMutate(coverageSwrKey)] : []),
      ]);
    } catch (err) {
      logger.error("revalidate_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        "Ansicht konnte nicht aktualisiert werden. Bitte die Seite neu laden.",
      );
    }
  }, [
    coverageSwrKey,
    gapsSwrKey,
    loadCoverage,
    weekSwrKey,
    tenantMutate,
    toast,
  ]);

  // Retry der Fehlerfläche: ein Backend-Blip lässt typischerweise alle drei
  // Abrufe gleichzeitig scheitern, also alle drei Keys revalidieren — sonst
  // bleiben Gaps-Chips und Personal-Alert stale, bis die Seite neu geladen
  // wird. allSettled statt all: Fehler tauchen über die error-States der
  // jeweiligen useSWRAuth-Hooks wieder auf, nicht als unhandled rejection.
  const retryAll = useCallback(() => {
    void Promise.allSettled([
      tenantMutate(weekSwrKey),
      tenantMutate(gapsSwrKey),
      tenantMutate(VERTRETUNG_STAFF_LIST_KEY),
      ...(loadCoverage ? [tenantMutate(coverageSwrKey)] : []),
    ]);
  }, [coverageSwrKey, gapsSwrKey, loadCoverage, weekSwrKey, tenantMutate]);

  // Ein atomares Save für das gesamte Editor-Formular. Der Cache-Refresh läuft
  // NACH der committeten Mutation und kann ihren Erfolg nicht zurücknehmen.
  const handleApply = useCallback(
    async (input: ApplyDeviationsInput): Promise<boolean> => {
      const instance = selectedInstance;
      if (!instance) return false;
      try {
        const result = await timetableService.applyDeviations(
          instance.id,
          input,
        );
        if (result.cancelled) {
          toast.success(
            cancelledToast("Block abgesagt", result.guardianNotice),
          );
          updateUrlParams({ block: null, verlauf: null });
        } else {
          toast.success(deviationSuccessMessage(input, result, staffNames));
          if (result.warnings.length > 0) {
            toast.error(
              `${result.warnings.length} mögliche Zeitüberschneidung(en) prüfen.`,
            );
          }
        }
        return true;
      } catch (err) {
        // Fehler an den Editor signalisieren (false zurückgeben, nicht werfen),
        // damit er das Formular mit den Eingaben OFFEN hält.
        logger.error("apply_deviations_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(
          err instanceof Error ? err.message : "Speichern fehlgeschlagen",
        );
        return false;
      } finally {
        await revalidate();
      }
    },
    [selectedInstance, revalidate, staffNames, toast, updateUrlParams],
  );

  // Solange die Session oder das Settings-Schema lädt, ist noch nicht
  // entscheidbar, ob das Feature aktiv ist und welche Aktionen erlaubt sind.
  // Wie im Dienstplan rendert die PlanningContextBar (Titel, Navigation)
  // trotzdem sofort; nur der Inhaltsbereich fällt auf das Skelett zurück.
  const showSkeleton = status === "loading" || settingsSchemaLoading;

  if (!showSkeleton && timetableDisabled) {
    return <VertretungDisabledState />;
  }

  return (
    <div className="w-full space-y-4">
      <PlanningContextBar
        title="Vertretung"
        onPrevious={() => goToDay(shiftDayISO(dayISO, -7))}
        onNext={() => goToDay(shiftDayISO(dayISO, 7))}
        previousLabel="Vorherige Woche"
        nextLabel="Nächste Woche"
        onToday={
          // Tagesansicht: der Button erscheint, sobald ein anderer Tag als das
          // Heute-Ziel gezeigt wird. Wochenansicht: sobald das Heute-Ziel gar
          // nicht in der sichtbaren Woche liegt.
          (
            isWeekView
              ? todayTarget < fromISO || todayTarget > toISO
              : dayISO !== todayTarget
          )
            ? () => goToDay(todayTarget)
            : undefined
        }
        dateLabel={weekLabel}
        viewSwitcher={
          <Tabs
            value={view}
            onValueChange={(v) => setView(v as VertretungViewMode)}
          >
            <TabsList variant="default">
              <TabsTrigger value="tag">Tag</TabsTrigger>
              <TabsTrigger value="woche">Woche</TabsTrigger>
            </TabsList>
          </Tabs>
        }
        actions={
          // Sammel-Vertretung (#2284): mehrtägige Abwesenheit + Ersatz in
          // einem Schritt. Reine Mutation, daher nur mit schedules:manage.
          canManageSchedules ? (
            <Button
              type="button"
              variant="primary"
              size="md"
              className="max-sm:h-8 max-sm:w-8 max-sm:justify-center max-sm:p-0"
              aria-label="Sammel-Vertretung eintragen"
              onClick={() => setBulkOpen(true)}
            >
              <CalendarRange
                className="h-4 w-4 shrink-0 sm:mr-1.5"
                aria-hidden
              />
              <span className="hidden whitespace-nowrap sm:inline">
                Sammel-Vertretung
              </span>
            </Button>
          ) : undefined
        }
      >
        {/* Die Tagesleiste steht in der Kontextzeile, nicht zwischen den
            Pfeilen: sie wählt einen Tag INNERHALB der Woche, die oben schon
            benannt ist. In der Wochenansicht entfällt sie, weil eine
            Tagesauswahl dort keine sichtbare Wirkung hätte — die Lücken pro Tag
            zeigen dann die Kopfzeilen der Wochenliste. */}
        {!isWeekView && (
          <>
            {weekDays.map((day) => {
              const iso = toISODate(day);
              const isPast = iso < today;
              // Vergangene Tage und ein fehlgeschlagener/übersprungener/noch
              // ladender Gaps-Abruf bekommen GAR KEINEN Zähler, nie eine
              // erfundene 0 (Akzeptanzkriterium 5). Der Chip bleibt dann still.
              const countUnavailable = isPast || !gapsLoaded;
              return (
                <PlanningDayChip
                  key={iso}
                  weekdayLabel={getGermanWeekdayShort(day)}
                  dateLabel={dayChipLabel(day)}
                  count={
                    countUnavailable ? undefined : (gapsByDate.get(iso) ?? 0)
                  }
                  selected={iso === dayISO}
                  onClick={() => goToDay(iso)}
                  aria-label={`${getGermanWeekdayShort(day)} ${dayChipLabel(day)}`}
                />
              );
            })}
            <span aria-hidden className="h-4 w-px bg-gray-200" />
          </>
        )}
        <CoverageIndicator
          size="sm"
          state={!countsUnavailable && openCount > 0 ? "gap" : "covered"}
          label={`Offen: ${countsUnavailable ? "–" : openCount}`}
          title={
            countsUnavailable
              ? "Offene Lücken nicht verfügbar"
              : `${openCount} offene Lücke(n) ${countScope}`
          }
        />
        <CoverageIndicator
          size="sm"
          state={
            !countsUnavailable && ackCount > 0 ? "acknowledged" : "covered"
          }
          label={`Quittiert: ${countsUnavailable ? "–" : ackCount}`}
          title={
            countsUnavailable
              ? "Quittierte Lücken nicht verfügbar"
              : `${ackCount} bewusst unbesetzt ${countScope}`
          }
        />
      </PlanningContextBar>

      {staffErrorMessage && (
        <Alert
          type="error"
          message={`Personalliste konnte nicht geladen werden: ${staffErrorMessage}. Ersatz kann nicht ausgewählt werden, bis die Seite neu geladen wurde.`}
        />
      )}

      {/* Planungshinweis, keine Störung: steht bewusst außerhalb der Liste und
          außerhalb der Zähler (siehe vertretung-coverage-notice.tsx). */}
      <VertretungCoverageNotice
        count={uncoveredCount}
        dienstplanHref={dienstplanHref}
      />

      {showSkeleton ? (
        <VertretungContentSkeleton />
      ) : weekErrorMessage ? (
        // Fehlerfläche mit Retry — NIE ein leerer Plan (Verhaltensvertrag).
        <div
          data-testid="vertretung-week-error"
          className={`${timetableSurface} space-y-3 p-4 sm:p-6`}
        >
          <Alert
            type="error"
            title="Vertretung konnte nicht geladen werden"
            message="Die Termine des Tages konnten nicht abgerufen werden. Bitte erneut versuchen."
          />
          <Button type="button" variant="outline" size="md" onClick={retryAll}>
            Erneut versuchen
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,400px)_minmax(0,1fr)]">
          {isWeekView ? (
            // Die Wochenliste darf die Seite nicht länger machen als das
            // Raster daneben. Ab lg trägt sie deshalb nichts zur Zeilenhöhe
            // bei (absolut positioniert), füllt die vom Raster gesetzte Höhe
            // und scrollt in sich — ohne die Rasterhöhe als Zahl zu kennen.
            // Darunter (einspaltig, ohne Raster) fließt sie normal und
            // scrollt mit der Seite.
            <div className="lg:relative">
              <div className="lg:absolute lg:inset-0">
                <VertretungWeekList
                  weekDays={weekDays}
                  instances={weekdayInstances}
                  gaps={weekGaps}
                  acknowledged={weekAcknowledged}
                  gapsAvailableFrom={gapsAvailableFrom}
                  staffNames={staffNames}
                  mode={mode}
                  onModeChange={setMode}
                  canManage={canManageSchedules}
                  onEdit={openEditor}
                  onSelectDay={openDayView}
                  todayISO={today}
                  className="lg:h-full lg:overflow-hidden"
                />
              </div>
            </div>
          ) : (
            <VertretungDayList
              instances={dayInstances}
              gaps={dayGaps}
              acknowledged={dayAcknowledged}
              gapsAvailable={!gapsUnavailable}
              staffNames={staffNames}
              mode={mode}
              onModeChange={setMode}
              canManage={canManageSchedules}
              onEdit={openEditor}
            />
          )}
          {/* Das Raster läuft auch unterhalb lg mit, dort einspaltig unter der
              Liste: es war früher desktop-only, wodurch die Vertretung mobil
              weniger zeigte als am Rechner — anders als der Betreuungsplan, der
              dasselbe Raster auf demselben Gerät sehr wohl darstellt.
              WeeklyCalendarGrid bringt die mobile Tagesauswahl selbst mit
              (eigener Tagesstreifen, alle übrigen Spalten unter sm verborgen),
              deshalb braucht es hier keine zweite Umschaltmechanik. In der
              Wochenansicht bekommt es fünf Tagesspalten statt einer. */}
          <div>
            <WeeklyCalendarGrid
              weekDays={isWeekView ? weekDays : [parseISODate(dayISO)]}
              instances={isWeekView ? weekdayInstances : dayInstances}
              selectedId={selectedInstanceId}
              onInstanceClick={(inst) => openEditor(inst.id)}
              gapInstanceIds={
                isWeekView ? weekGapInstanceIds : dayGapInstanceIds
              }
              todayISO={today}
              dayStartHour={dayStartHour}
              dayEndHour={dayEndHour}
              hourHeightPx={HOUR_HEIGHT_PX}
              emptyState={
                (isWeekView ? weekdayInstances : dayInstances).length > 0
                  ? undefined
                  : isLoading
                    ? { title: "Lädt…", description: "Termine werden geladen." }
                    : {
                        title: "Keine Termine",
                        description: isWeekView
                          ? "Für diese Woche sind keine Termine geplant."
                          : "Für diesen Tag sind keine Termine geplant.",
                      }
              }
            />
          </div>
        </div>
      )}

      <BulkSubstitutionModal
        isOpen={bulkOpen}
        onClose={() => setBulkOpen(false)}
        staffOptions={staffOptions}
        staffLoadError={staffErrorMessage !== null}
        onSaved={revalidate}
      />

      <SubstitutionSlideOver
        instance={selectedInstance}
        dayInstances={editorDayInstances}
        staffOptions={staffOptions}
        staffNames={staffNames}
        staffLoadError={staffErrorMessage !== null}
        canManage={canManageSchedules}
        onClose={closeEditor}
        onApply={handleApply}
        initialTab={historyOpen ? "verlauf" : "bearbeiten"}
        onTabChange={(tab) =>
          updateUrlParams({ verlauf: tab === "verlauf" ? "1" : null })
        }
      />
    </div>
  );
}

function VertretungDisabledState() {
  return (
    <PlanningDisabledState
      pageTitle="Vertretung"
      heading="Vertretung ist deaktiviert"
      description="Die Vertretung gehört zum Betreuungsplan, der für diese Schule ausgeschaltet ist. Er kann in den Einstellungen unter „Betrieb“ wieder aktiviert werden."
      testId="vertretung-disabled-state"
    />
  );
}

/**
 * VertretungView ist der einbettbare Vertretungs-Zweiteiler, gehostet von
 * /vertretung.
 */
export function VertretungView() {
  return (
    <Suspense fallback={<VertretungContentSkeleton withBar />}>
      <VertretungContent />
    </Suspense>
  );
}
