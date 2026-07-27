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
import { SubstitutionSlideOver } from "~/components/timetable/substitution-slide-over";
import {
  VertretungDayList,
  type VertretungDayListMode,
} from "~/components/timetable/vertretung-day-list";
import { VertretungWeekList } from "~/components/timetable/vertretung-week-list";
import { timetableSurface } from "~/components/timetable/timetable-style";
import { WeeklyCalendarGrid } from "~/components/timetable/weekly-calendar-grid";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
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
import { staffService } from "~/lib/staff-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
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
import type { ApplyDeviationsInput } from "~/lib/timetable-types";

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
  // Personen, die irgendwo am Tag der selektierten Instanz abwesend sind.
  // Abwesenheit ist tagesweit, daher sind diese Personen aus jeder
  // Ersatz-Auswahl auszuschließen (Abschnitt 3, Muster der alten View).
  const dayAbsentStaffIds = useMemo(() => {
    const ids = new Set<string>();
    const date = selectedInstance?.date;
    if (!date) return ids;
    for (const inst of instances) {
      if (inst.date !== date) continue;
      for (const row of inst.staff) {
        if (row.isAbsent) ids.add(row.staffId);
      }
    }
    return ids;
  }, [instances, selectedInstance?.date]);

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
  const gapsAvailableFrom = gapsLoaded ? gapsFromISO : null;

  // Zähler der Kontextleiste: die Tagesansicht zählt den Tag, die
  // Wochenansicht die geladene Woche. Ein auf heute geklemmtes Fenster ist
  // keine vollständige Wochenaussage — das steht im Tooltip, statt eine
  // vollständige Zahl vorzutäuschen.
  const countsUnavailable = isWeekView ? !gapsLoaded : gapsUnavailable;
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

  // Filterzustand ist LOKAL, kein URL-Parameter (verbindliches
  // Drei-Parameter-Vokabular). Standard "Nur Störungen" (aus K2).
  const [mode, setMode] = useState<VertretungDayListMode>("stoerungen");

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
      await Promise.all([tenantMutate(weekSwrKey), tenantMutate(gapsSwrKey)]);
    } catch (err) {
      logger.error("revalidate_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        "Ansicht konnte nicht aktualisiert werden. Bitte die Seite neu laden.",
      );
    }
  }, [gapsSwrKey, weekSwrKey, tenantMutate, toast]);

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
    ]);
  }, [gapsSwrKey, weekSwrKey, tenantMutate]);

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
          toast.success("Block abgesagt");
          updateUrlParams({ block: null, verlauf: null });
        } else {
          const count = result.affectedInstances.length;
          toast.success(
            count > 0
              ? `Vertretung aktualisiert: ${count} Termin(e)`
              : "Vertretung aktualisiert",
          );
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
    [selectedInstance, revalidate, toast, updateUrlParams],
  );

  // Solange das Settings-Schema lädt, ist noch nicht entscheidbar, ob das
  // Feature aktiv ist — nichts rendern statt kurz die Seite aufblitzen zu lassen.
  if (status === "loading" || settingsSchemaLoading) {
    return null;
  }

  if (timetableDisabled) {
    return <VertretungDisabledState />;
  }

  return (
    <div className="flex flex-col gap-4">
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
        dateLabel={isWeekView ? weekLabel : undefined}
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
        navigationSlot={
          // In der Wochenansicht ersetzt das Wochenlabel die Tagesleiste: eine
          // Tagesauswahl hätte dort keine sichtbare Wirkung. Die Lücken pro Tag
          // zeigen dann die Kopfzeilen der Wochenliste.
          isWeekView ? undefined : (
            <div className="flex items-center gap-0.5">
              {weekDays.map((day) => {
                const iso = toISODate(day);
                const isPast = iso < today;
                // Vergangene Tage und ein fehlgeschlagener/übersprungener/noch
                // ladender Gaps-Abruf zeigen einen Platzhalter, nie eine
                // erfundene 0 (Akzeptanzkriterium 5).
                const showPlaceholder = isPast || !gapsLoaded;
                return (
                  <PlanningDayChip
                    key={iso}
                    weekdayLabel={getGermanWeekdayShort(day)}
                    dateLabel={dayChipLabel(day)}
                    count={
                      showPlaceholder ? undefined : (gapsByDate.get(iso) ?? 0)
                    }
                    showPlaceholder={showPlaceholder}
                    selected={iso === dayISO}
                    onClick={() => goToDay(iso)}
                    aria-label={`${getGermanWeekdayShort(day)} ${dayChipLabel(day)}`}
                  />
                );
              })}
            </div>
          )
        }
      >
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
        <Tabs
          value={mode}
          onValueChange={(v) => setMode(v as VertretungDayListMode)}
          className="ml-auto"
        >
          <TabsList variant="default">
            <TabsTrigger value="stoerungen">Nur Störungen</TabsTrigger>
            <TabsTrigger value="ganzer-tag">
              {isWeekView ? "Ganze Woche" : "Ganzer Tag"}
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </PlanningContextBar>

      {staffErrorMessage && (
        <Alert
          type="error"
          message={`Personalliste konnte nicht geladen werden: ${staffErrorMessage}. Ersatz kann nicht ausgewählt werden, bis die Seite neu geladen wurde.`}
        />
      )}

      {weekErrorMessage ? (
        // Fehlerfläche mit Retry — NIE ein leerer Plan (Verhaltensvertrag).
        <div
          data-testid="vertretung-week-error"
          className={`${timetableSurface} flex flex-col items-center gap-3 p-8 text-center`}
        >
          <p className="text-sm font-semibold text-gray-900">
            Vertretung konnte nicht geladen werden
          </p>
          <p className="max-w-md text-sm leading-relaxed text-gray-600">
            Die Termine des Tages konnten nicht abgerufen werden. Bitte erneut
            versuchen.
          </p>
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
              canManage={canManageSchedules}
              onEdit={openEditor}
            />
          )}
          {/* Unterhalb lg entfällt die Kalenderspalte; die Liste bleibt allein
              voll funktionsfähig (Abschnitt 2). Das Raster ist dasselbe, es
              bekommt in der Wochenansicht nur fünf Tagesspalten statt einer. */}
          <div className="hidden lg:block">
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

      <SubstitutionSlideOver
        instance={selectedInstance}
        staffOptions={staffOptions}
        staffNames={staffNames}
        dayAbsentStaffIds={dayAbsentStaffIds}
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
    <Suspense fallback={null}>
      <VertretungContent />
    </Suspense>
  );
}
