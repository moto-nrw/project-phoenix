"use client";

// Aufsichten der Lehrkraft im Schul-Portal ("moto schule", #2527).
//
// Die Seite hat zwei Zustände, und der Tag bestimmt sie: läuft eine Aufsicht,
// IST die Seite diese Aufsicht — Kinderliste, sonst nichts. Läuft keine, zeigt
// sie den Tag als schlichte Liste. Es gibt bewusst kein Auswahl-Bedienelement:
// eine Lehrkraft macht immer die Aufsicht, die gerade dran ist, und muss das
// nicht erst angeben. Nach dem Beenden fällt die Seite von selbst in den
// Überblick zurück.
//
// Das Backend filtert auf die eigene Einteilung im Betreuungsplan; die
// Oberfläche blendet nichts aus. Die Kinderliste ist dieselbe Komponente, die
// eine Betreuungskraft im OGS-Portal bedient, nur ohne das Nachtragen fremder
// Kinder: dafür hat eine Lehrkraft keine Kindersuche und keine Berechtigung.

import { ChevronLeft } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import {
  TimetableRosterContent,
  type RosterAction,
} from "~/components/active-supervisions/timetable-roster";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { schoolSupervisionsApi } from "~/lib/school-supervisions-api";
import { useSWRAuth } from "~/lib/swr";
import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { berlinTodayISO } from "~/lib/date-helpers";
import { StudentSheetModal } from "./student-sheet-modal";
import { SupervisionRosterPreview } from "./roster-preview";
import { SupervisionsOverview } from "./supervisions-overview";
import {
  AUTO_VIEW,
  resolveSupervisionView,
  supervisionStartState,
  upcomingAfter,
  type SupervisionStartState,
  type SupervisionViewIntent,
} from "./view-model";

const logger = createLogger({ component: "SchoolSupervisionsView" });

const GENERIC_ERROR =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";

/**
 * Eine Aufsicht, die nicht läuft: ein Satz, was als Nächstes passiert, und
 * genau ein Knopf. Mehr gibt es an dieser Stelle nicht zu entscheiden.
 */
const START_EXPLANATIONS: Record<SupervisionStartState, string> = {
  cancelled: "Dieser Termin findet heute nicht statt. Sie müssen nichts tun.",
  completed: "Diese Aufsicht ist beendet.",
  startable:
    "Starten Sie die Aufsicht, wenn die Kinder da sind. Danach können Sie die Anwesenheit führen.",
  too_early: "",
  expired:
    "Diese Aufsicht ist vorbei und wurde nicht gestartet. Melden Sie sich im OGS-Büro, wenn die Anwesenheit noch nachgetragen werden soll.",
};

function SupervisionIntro({
  instance,
  busy,
  onStart,
}: Readonly<{
  instance: PlannedTimetableInstance;
  busy: boolean;
  onStart: () => void;
}>) {
  const room = instance.roomName ?? `Raum ${instance.roomId}`;
  // Die Minutenuhr, weil "noch nicht" von selbst in "jetzt" übergehen muss,
  // ohne dass jemand die Seite neu lädt.
  const now = useMinuteClock();
  const startState = supervisionStartState(instance, now);
  const explanation =
    startState === "too_early"
      ? `Die Aufsicht lässt sich ab ${instance.startTime} starten.`
      : START_EXPLANATIONS[startState];
  const cancelled = startState === "cancelled";
  const showStartButton =
    startState === "startable" || startState === "too_early";

  return (
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold text-gray-900">
            {instance.title}
          </h2>
          <p className="mt-1 truncate text-sm text-gray-600">
            {room} · {instance.startTime} bis {instance.endTime}
          </p>
        </div>
        {cancelled ? (
          <StatusDotBadge label="Fällt aus" color={LOCATION_COLORS.DANGER} />
        ) : null}
      </div>

      <p className="mt-3 max-w-2xl text-sm leading-6 text-gray-600">
        {explanation}
      </p>

      {showStartButton ? (
        <div className="mt-4">
          <Button
            type="button"
            size="md"
            variant="success"
            disabled={busy || startState !== "startable"}
            onClick={onStart}
          >
            {busy ? "Wird gestartet..." : "Aufsicht starten"}
          </Button>
        </div>
      ) : null}

      {instance.isSubstitute ? (
        <p className="mt-3 text-xs text-gray-500">
          Sie vertreten hier eine andere Person.
        </p>
      ) : null}
    </section>
  );
}

export function SchoolSupervisionsView() {
  const today = berlinTodayISO();

  const [intent, setIntent] = useState<SupervisionViewIntent>(AUTO_VIEW);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sheetRow, setSheetRow] = useState<TimetableRosterRow | null>(null);

  const {
    data: instances,
    isLoading,
    error: listError,
    mutate: reloadList,
  } = useSWRAuth(
    `school-supervisions-${today}`,
    () => schoolSupervisionsApi.myDay(),
    { revalidateOnFocus: true, focusThrottleInterval: 60_000 },
  );

  const sortedInstances = useMemo(
    () =>
      [...(instances ?? [])].sort((a, b) =>
        a.startTime.localeCompare(b.startTime),
      ),
    [instances],
  );

  const view = useMemo(
    () => resolveSupervisionView(intent, sortedInstances),
    [intent, sortedInstances],
  );
  const openInstance = view.mode === "detail" ? view.instance : null;
  const openId = openInstance?.id ?? null;

  // Ein Wechsel der Aufsicht schließt ein offenes Kind-Infoblatt: es gehört
  // zu einem Kind der vorherigen Liste.
  useEffect(() => {
    setSheetRow(null);
  }, [openId]);

  const showRoster =
    openInstance?.status === "active" || openInstance?.status === "completed";

  // Auch fuer einen geplanten Block: die Frage vor dem Start ist "wer kommt
  // gleich", und der Server beantwortet sie. Nur ein abgesagter Termin hat
  // keine Liste, die jemanden interessiert.
  const wantsRoster =
    openInstance != null && openInstance.status !== "cancelled";

  const {
    data: roster,
    isLoading: rosterLoading,
    mutate: reloadRoster,
  } = useSWRAuth(
    openId && wantsRoster ? `school-supervision-roster-${openId}` : null,
    () => schoolSupervisionsApi.roster(openId as string),
    { keepPreviousData: false, revalidateOnFocus: false },
  );

  const report = useCallback((event: string, err: unknown) => {
    setError(GENERIC_ERROR);
    logger.error(event, {
      error: err instanceof Error ? err.message : String(err),
    });
  }, []);

  const refreshAll = useCallback(async () => {
    await Promise.all([reloadList(), reloadRoster()]);
  }, [reloadList, reloadRoster]);

  const handleStart = useCallback(
    async (instance: PlannedTimetableInstance) => {
      setBusyId(instance.id);
      setError(null);
      try {
        await schoolSupervisionsApi.start(instance.id);
        // Zurück auf "der Tag entscheidet": die Aufsicht läuft jetzt und ist
        // damit von selbst die, die die Seite zeigt.
        setIntent(AUTO_VIEW);
        await Promise.all([reloadList(), reloadRoster()]);
      } catch (err) {
        report("supervision_start_failed", err);
      } finally {
        setBusyId(null);
      }
    },
    [reloadList, reloadRoster, report],
  );

  const handleRosterAction = useCallback(
    async (action: RosterAction, row: TimetableRosterRow) => {
      if (!openId) return;
      setError(null);
      try {
        if (action === "check-in") {
          await schoolSupervisionsApi.checkIn(openId, row.studentId);
        } else if (action === "check-out") {
          await schoolSupervisionsApi.checkOut(openId, row.studentId);
        } else if (action === "expected") {
          await schoolSupervisionsApi.patchAttendance(openId, row.studentId, {
            status: "expected",
            substatus: null,
            note: null,
          });
        } else {
          await schoolSupervisionsApi.patchAttendance(
            openId,
            row.studentId,
            action === "excused"
              ? { status: "absent", substatus: "excused" }
              : { status: "absent" },
          );
        }
        await refreshAll();
      } catch (err) {
        report("supervision_roster_action_failed", err);
      }
    },
    [openId, refreshAll, report],
  );

  const handleConfirmExpected = useCallback(
    async (rows: TimetableRosterRow[]) => {
      if (!openId) return;
      setError(null);
      try {
        for (const row of rows) {
          await schoolSupervisionsApi.checkIn(openId, row.studentId);
        }
        await refreshAll();
      } catch (err) {
        report("supervision_confirm_expected_failed", err);
      }
    },
    [openId, refreshAll, report],
  );

  const handleComplete = useCallback(async () => {
    if (!openId || !roster) return;
    setError(null);
    setBusyId(openId);
    try {
      const present = roster.rows
        .filter((row) => row.currentlyPresent)
        .map((row) => row.studentId);
      await schoolSupervisionsApi.complete(openId, present);
      // Nichts läuft mehr, also zeigt die Seite wieder den Tag.
      setIntent(AUTO_VIEW);
      await reloadList();
    } catch (err) {
      report("supervision_complete_failed", err);
    } finally {
      setBusyId(null);
    }
  }, [openId, reloadList, report, roster]);

  const rosterMatchesSelection =
    roster != null && openId != null && roster.instance.id === openId;

  const upcoming = openInstance
    ? upcomingAfter(openInstance, sortedInstances)
    : [];

  return (
    <div className="w-full space-y-4">
      {error ? <Alert type="error" message={error} /> : null}
      {listError ? (
        <Alert
          type="error"
          message="Ihre Aufsichten konnten nicht geladen werden. Bitte laden Sie die Seite neu."
        />
      ) : null}

      {isLoading ? <Skeleton className="h-64 w-full" /> : null}

      {!isLoading && view.mode === "overview" ? (
        <SupervisionsOverview
          instances={sortedInstances}
          today={today}
          onOpen={(id) => setIntent({ kind: "detail", id })}
        />
      ) : null}

      {view.mode === "detail" && view.canGoBack ? (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          className="-ml-2"
          onClick={() => setIntent({ kind: "overview" })}
        >
          <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          Alle Aufsichten heute
        </Button>
      ) : null}

      {openInstance && !showRoster ? (
        <SupervisionIntro
          instance={openInstance}
          busy={busyId === openInstance.id}
          onStart={() => void handleStart(openInstance)}
        />
      ) : null}

      {openInstance &&
      !showRoster &&
      wantsRoster &&
      rosterLoading &&
      !rosterMatchesSelection ? (
        <Skeleton className="h-48 w-full" />
      ) : null}

      {openInstance && !showRoster && rosterMatchesSelection ? (
        <SupervisionRosterPreview
          rows={roster.rows}
          pickupTimesLoaded={roster.pickupTimesLoaded}
          onOpenStudent={setSheetRow}
        />
      ) : null}

      {openInstance &&
      showRoster &&
      rosterLoading &&
      !rosterMatchesSelection ? (
        <Skeleton className="h-64 w-full" />
      ) : null}

      {openInstance && showRoster && rosterMatchesSelection ? (
        <TimetableRosterContent
          roster={roster as TimetableRoster}
          attendanceWebEnabled={openInstance.status === "active"}
          showTimetableCounts
          canAddUnplanned={false}
          // Ohne diesen Satz ist der unterstrichene Name nur ein
          // unterstrichener Name — die Abhol- und Notfallangaben hinter dem
          // Antippen findet sonst niemand.
          headerNote="Tippen Sie auf einen Namen. Sie sehen dann, wann das Kind geht, wer es abholen darf und wen Sie im Notfall anrufen."
          addStudentResults={[]}
          addStudentSearch=""
          isAddingStudent={false}
          isCompletingInstance={busyId === openId}
          isConfirmingExpected={false}
          onAddStudent={() => Promise.resolve(false)}
          onSearchChange={() => undefined}
          onRosterAction={handleRosterAction}
          onConfirmExpected={handleConfirmExpected}
          onComplete={handleComplete}
          onOpenStudent={setSheetRow}
        />
      ) : null}

      {/* Die einzige andere Frage waehrend einer Aufsicht: was kommt danach.
          Ein Satz, kein Bedienelement. */}
      {openInstance && upcoming.length > 0 ? (
        <p className="px-1 text-sm text-gray-500">
          Danach:{" "}
          {upcoming
            .map((item) => `${item.title} ab ${item.startTime}`)
            .join(", ")}
        </p>
      ) : null}

      {openId ? (
        <StudentSheetModal
          instanceId={openId}
          studentId={sheetRow?.studentId ?? null}
          studentName={sheetRow?.studentName ?? ""}
          onClose={() => setSheetRow(null)}
        />
      ) : null}
    </div>
  );
}
