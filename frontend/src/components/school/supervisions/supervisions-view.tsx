"use client";

// Aufsichten der Lehrkraft im Schul-Portal ("moto schule", #2527).
//
// Die Liste zeigt genau die Betreuungsplan-Blöcke des heutigen Tages, in die
// diese Lehrkraft eingeteilt ist — das Backend filtert, die Oberfläche blendet
// nichts aus. Ist ein Block offen, rendert darunter dieselbe Kinderliste, die
// eine Betreuungskraft im OGS-Portal bedient (TimetableRosterContent), nur
// ohne das Nachtragen fremder Kinder: dafür hat eine Lehrkraft keine
// Kindersuche und keine Berechtigung.

import { useCallback, useMemo, useState } from "react";
import type { CSSProperties } from "react";
import { useSession } from "next-auth/react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StatTile } from "~/components/ui/stat-tile";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import {
  TimetableRosterContent,
  type RosterAction,
} from "~/components/active-supervisions/timetable-roster";
import { getUserDisplayName } from "~/lib/auth-utils";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { getTimeBasedGreeting } from "~/lib/greeting";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { schoolSupervisionsApi } from "~/lib/school-supervisions-api";
import { useSWRAuth } from "~/lib/swr";
import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { StudentSheetModal } from "./student-sheet-modal";

const logger = createLogger({ component: "SchoolSupervisionsView" });

const GENERIC_ERROR =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";

const STATUS_LABELS: Record<PlannedTimetableInstance["status"], string> = {
  planned: "Noch nicht gestartet",
  active: "Läuft",
  completed: "Beendet",
  cancelled: "Fällt aus",
};

const STATUS_COLORS: Record<PlannedTimetableInstance["status"], string> = {
  planned: LOCATION_COLORS.UNKNOWN,
  active: LOCATION_COLORS.GROUP_ROOM,
  completed: LOCATION_COLORS.OTHER_ROOM,
  // DANGER, nicht SICK: der Termin faellt aus, das Kind ist nicht krank.
  // Gleicher Hex, aber der Name muss sagen, was gemeint ist.
  cancelled: LOCATION_COLORS.DANGER,
};

function SupervisionCard({
  instance,
  selected,
  busy,
  onOpen,
  onStart,
}: Readonly<{
  instance: PlannedTimetableInstance;
  selected: boolean;
  busy: boolean;
  onOpen: () => void;
  onStart: () => void;
}>) {
  const room = instance.roomName ?? `Raum ${instance.roomId}`;
  const running = instance.status === "active";
  const cancelled = instance.status === "cancelled";

  return (
    <div
      // Die Auswahlfarbe kommt als Variable aus der Marken-Tabelle, nicht als
      // Hex in einer Utility-Klasse: ein Literal bliebe stehen, wenn die
      // Palette sich bewegt.
      style={
        { "--supervision-accent": LOCATION_COLORS.OTHER_ROOM } as CSSProperties
      }
      className={`rounded-2xl border bg-white p-4 shadow-sm ${
        selected
          ? "border-[var(--supervision-accent)] ring-1 ring-[var(--supervision-accent)]"
          : "border-gray-200"
      }`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-gray-900">
            {instance.title}
          </h3>
          <p className="mt-1 truncate text-xs text-gray-500">
            {room} · {instance.startTime} bis {instance.endTime}
          </p>
        </div>
        <StatusDotBadge
          label={STATUS_LABELS[instance.status]}
          color={STATUS_COLORS[instance.status]}
        />
      </div>

      {!cancelled ? (
        <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-3">
          <StatTile label="Erwartet" value={instance.expectedStudentsCount} />
          <StatTile label="Anwesend" value={instance.presentStudentsCount} />
          {instance.notScheduledStudentsCount > 0 ? (
            <StatTile
              label="Heute nicht eingeplant"
              value={instance.notScheduledStudentsCount}
            />
          ) : null}
        </div>
      ) : null}

      <div className="mt-3 flex flex-wrap gap-2">
        {instance.status === "planned" ? (
          <Button
            type="button"
            size="md"
            variant="success"
            disabled={busy || instance.canStart === false}
            onClick={onStart}
          >
            {instance.canStart === false
              ? `Start ab ${instance.startTime}`
              : "Aufsicht starten"}
          </Button>
        ) : null}
        {running || instance.status === "completed" ? (
          <Button type="button" size="md" variant="outline" onClick={onOpen}>
            {selected ? "Liste ausblenden" : "Kinderliste öffnen"}
          </Button>
        ) : null}
      </div>

      {instance.isSubstitute ? (
        <p className="mt-2 text-xs text-gray-500">
          Sie vertreten hier eine andere Person.
        </p>
      ) : null}
      {cancelled ? (
        <p className="mt-2 text-xs text-gray-500">
          Dieser Termin findet heute nicht statt.
        </p>
      ) : null}
    </div>
  );
}

export function SchoolSupervisionsView() {
  const { data: session } = useSession();
  const today = berlinTodayISO();

  const [selectedId, setSelectedId] = useState<string | null>(null);
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

  const {
    data: roster,
    isLoading: rosterLoading,
    mutate: reloadRoster,
  } = useSWRAuth(
    selectedId ? `school-supervision-roster-${selectedId}` : null,
    () => schoolSupervisionsApi.roster(selectedId as string),
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
        setSelectedId(instance.id);
        await reloadList();
      } catch (err) {
        report("supervision_start_failed", err);
      } finally {
        setBusyId(null);
      }
    },
    [reloadList, report],
  );

  const handleRosterAction = useCallback(
    async (action: RosterAction, row: TimetableRosterRow) => {
      if (!selectedId) return;
      setError(null);
      try {
        if (action === "check-in") {
          await schoolSupervisionsApi.checkIn(selectedId, row.studentId);
        } else if (action === "check-out") {
          await schoolSupervisionsApi.checkOut(selectedId, row.studentId);
        } else if (action === "expected") {
          await schoolSupervisionsApi.patchAttendance(
            selectedId,
            row.studentId,
            { status: "expected", substatus: null, note: null },
          );
        } else {
          await schoolSupervisionsApi.patchAttendance(
            selectedId,
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
    [refreshAll, report, selectedId],
  );

  const handleConfirmExpected = useCallback(
    async (rows: TimetableRosterRow[]) => {
      if (!selectedId) return;
      setError(null);
      try {
        for (const row of rows) {
          await schoolSupervisionsApi.checkIn(selectedId, row.studentId);
        }
        await refreshAll();
      } catch (err) {
        report("supervision_confirm_expected_failed", err);
      }
    },
    [refreshAll, report, selectedId],
  );

  const handleComplete = useCallback(async () => {
    if (!selectedId || !roster) return;
    setError(null);
    try {
      const present = roster.rows
        .filter((row) => row.currentlyPresent)
        .map((row) => row.studentId);
      await schoolSupervisionsApi.complete(selectedId, present);
      setSelectedId(null);
      await reloadList();
    } catch (err) {
      report("supervision_complete_failed", err);
    }
  }, [reloadList, report, roster, selectedId]);

  const rosterMatchesSelection =
    roster != null && selectedId != null && roster.instance.id === selectedId;

  const sortedInstances = useMemo(
    () =>
      [...(instances ?? [])].sort((a, b) =>
        a.startTime.localeCompare(b.startTime),
      ),
    [instances],
  );

  const noSupervisions =
    !isLoading && !listError && sortedInstances.length === 0;

  return (
    <div className="w-full">
      <SectionCard
        kicker="Meine Aufsichten"
        title={`${getTimeBasedGreeting()}, ${getUserDisplayName(session)}`}
        description={`Ihre Aufsichten am ${formatDate(today)}. Sie sehen nur die Termine, für die Sie im Betreuungsplan eingeteilt sind.`}
      >
        {error ? (
          <div className="mb-4">
            <Alert type="error" message={error} />
          </div>
        ) : null}
        {listError ? (
          <div className="mb-4">
            <Alert
              type="error"
              message="Ihre Aufsichten konnten nicht geladen werden. Bitte laden Sie die Seite neu."
            />
          </div>
        ) : null}

        <div className="space-y-4">
          {isLoading ? (
            <div className="grid gap-3 lg:grid-cols-2">
              <Skeleton className="h-40 w-full" />
              <Skeleton className="h-40 w-full" />
            </div>
          ) : null}

          {noSupervisions ? (
            <EmptyState
              title="Heute keine Aufsicht für Sie"
              description="Für heute sind Sie im Betreuungsplan keiner Aufsicht zugeteilt. Die Einteilung macht das OGS-Büro."
            />
          ) : null}

          {sortedInstances.length > 0 ? (
            <div className="grid gap-3 lg:grid-cols-2">
              {sortedInstances.map((instance) => (
                <SupervisionCard
                  key={instance.id}
                  instance={instance}
                  selected={selectedId === instance.id}
                  busy={busyId === instance.id}
                  onOpen={() =>
                    setSelectedId((current) =>
                      current === instance.id ? null : instance.id,
                    )
                  }
                  onStart={() => void handleStart(instance)}
                />
              ))}
            </div>
          ) : null}
        </div>
      </SectionCard>

      {selectedId ? (
        <div className="mt-4">
          {rosterLoading && !rosterMatchesSelection ? (
            <Skeleton className="h-64 w-full" />
          ) : null}
          {rosterMatchesSelection ? (
            <TimetableRosterContent
              roster={roster as TimetableRoster}
              attendanceWebEnabled
              showTimetableCounts
              canAddUnplanned={false}
              // Ohne diesen Satz ist der unterstrichene Name nur ein
              // unterstrichener Name — die Abhol- und Notfallangaben hinter
              // dem Antippen findet sonst niemand.
              headerNote="Tippen Sie auf einen Namen. Sie sehen dann, wann das Kind geht, wer es abholen darf und wen Sie im Notfall anrufen."
              addStudentResults={[]}
              addStudentSearch=""
              isAddingStudent={false}
              isCompletingInstance={busyId === selectedId}
              isConfirmingExpected={false}
              onAddStudent={() => Promise.resolve(false)}
              onSearchChange={() => undefined}
              onRosterAction={handleRosterAction}
              onConfirmExpected={handleConfirmExpected}
              onComplete={handleComplete}
              onOpenStudent={setSheetRow}
            />
          ) : null}
        </div>
      ) : null}

      {selectedId ? (
        <StudentSheetModal
          instanceId={selectedId}
          studentId={sheetRow?.studentId ?? null}
          studentName={sheetRow?.studentName ?? ""}
          onClose={() => setSheetRow(null)}
        />
      ) : null}
    </div>
  );
}
