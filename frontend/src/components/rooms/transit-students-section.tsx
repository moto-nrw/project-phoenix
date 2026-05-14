"use client";

import { useMemo, useState } from "react";
import {
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
} from "~/lib/swr";
import { Alert } from "~/components/ui/alert";
import { activeService } from "~/lib/active-service";
import type { ActiveGroup } from "~/lib/active-helpers";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import { LOCATION_COLORS } from "~/lib/location-helper";

function buildSessionLabel(group: ActiveGroup): string {
  const roomName = group.room?.name ?? `Raum ${group.roomId}`;
  const activityName = group.actualGroup?.name;
  return activityName ? `${roomName} · ${activityName}` : roomName;
}

export function TransitStudentsSection() {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [targetGroupId, setTargetGroupId] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const mutateKey = useTenantMutate();
  const mutateMatching = useTenantMutateMatching([
    "room-students-",
    "room-detail-",
    "rooms-list",
    "dashboard",
  ]);

  const {
    data: studentsData,
    error: studentsError,
    isLoading: studentsLoading,
    mutate: mutateStudents,
  } = useSWRAuth<{
    students: Student[];
    pagination?: { total_records: number };
  }>("transit-students", () =>
    studentService.getStudents({ locationState: "transit", pageSize: 200 }),
  );

  const {
    data: activeGroups = [],
    error: groupsError,
    isLoading: groupsLoading,
  } = useSWRAuth<ActiveGroup[]>("active-groups-for-transit", () =>
    activeService.getActiveGroups({ active: true }),
  );

  const targetOptions = useMemo(
    () =>
      [...activeGroups]
        .filter((group) => group.isActive)
        .sort((a, b) =>
          buildSessionLabel(a).localeCompare(buildSessionLabel(b), "de"),
        ),
    [activeGroups],
  );

  const students = studentsData?.students ?? [];
  const totalCount = studentsData?.pagination?.total_records ?? students.length;

  const toggleSelected = (studentId: string) => {
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(studentId)) next.delete(studentId);
      else next.add(studentId);
      return next;
    });
    setMessage(null);
    setSubmitError(null);
  };

  const assignSelected = async () => {
    if (!targetGroupId || selectedIds.size === 0) return;
    setSubmitting(true);
    setMessage(null);
    setSubmitError(null);
    try {
      const result = await activeService.assignTransitStudents(
        Array.from(selectedIds),
        targetGroupId,
      );
      setSelectedIds(new Set());
      await mutateStudents();
      await mutateKey("rooms-list");
      await mutateMatching();
      const skipped = result.skipped.length;
      setMessage(
        skipped > 0
          ? `${result.assigned.length} zugewiesen, ${skipped} übersprungen.`
          : `${result.assigned.length} ${
              result.assigned.length === 1 ? "Kind" : "Kinder"
            } zugewiesen.`,
      );
    } catch {
      setSubmitError(
        "Die ausgewählten Kinder konnten nicht zugewiesen werden.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section
      aria-label="Kinder unterwegs"
      className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6"
    >
      <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
        Kinder unterwegs
      </h2>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <p className="text-sm text-gray-600">
          <span className="font-medium text-gray-900">{totalCount}</span>{" "}
          {totalCount === 1 ? "Kind" : "Kinder"} ohne Raum
        </p>
        <span
          className="rounded-full px-2.5 py-1 text-xs font-medium"
          style={{
            backgroundColor: `${LOCATION_COLORS.TRANSIT}1A`,
            color: LOCATION_COLORS.TRANSIT,
          }}
        >
          Sonderbereich
        </span>
      </div>

      {studentsError || groupsError ? (
        <div className="mt-4">
          <Alert
            type="error"
            message="Die Unterwegs-Daten konnten nicht geladen werden."
          />
        </div>
      ) : null}
      {submitError ? (
        <div className="mt-4">
          <Alert type="error" message={submitError} />
        </div>
      ) : null}
      {message ? (
        <div className="mt-4 rounded-lg border border-[#83CD2D]/30 bg-[#83CD2D]/10 p-3 text-sm text-[#4a7a15]">
          {message}
        </div>
      ) : null}

      <div className="mt-4 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto]">
        <select
          value={targetGroupId}
          onChange={(event) => setTargetGroupId(event.target.value)}
          disabled={groupsLoading || targetOptions.length === 0}
          className="min-h-10 rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 focus:border-[#5080D8] focus:ring-2 focus:ring-[#5080D8]/20 focus:outline-none disabled:bg-gray-50 disabled:text-gray-400"
        >
          <option value="">
            {groupsLoading
              ? "Aktive Räume werden geladen..."
              : targetOptions.length === 0
                ? "Keine aktiven Räume"
                : "Zielraum wählen"}
          </option>
          {targetOptions.map((group) => (
            <option key={group.id} value={group.id}>
              {buildSessionLabel(group)}
            </option>
          ))}
        </select>
        <button
          type="button"
          onClick={() => void assignSelected()}
          disabled={!targetGroupId || selectedIds.size === 0 || submitting}
          className="inline-flex min-h-10 items-center justify-center rounded-lg bg-[#5080D8] px-4 text-sm font-medium text-white transition-colors hover:bg-[#4070c8] disabled:cursor-not-allowed disabled:bg-gray-300"
        >
          {submitting
            ? "Zuweisen..."
            : `${selectedIds.size} ${
                selectedIds.size === 1 ? "Kind" : "Kinder"
              } zuweisen`}
        </button>
      </div>

      <div className="mt-4 flex flex-col gap-2">
        {studentsLoading && students.length === 0 ? (
          <div className="rounded-xl border border-gray-200 bg-white px-4 py-6 text-center text-sm text-gray-500">
            Kinder werden geladen...
          </div>
        ) : students.length === 0 ? (
          <div className="rounded-xl border border-dashed border-gray-200 bg-white px-4 py-6 text-center text-sm text-gray-500">
            Aktuell keine Kinder unterwegs.
          </div>
        ) : (
          students.map((student) => {
            const studentId = student.id.toString();
            const checked = selectedIds.has(studentId);
            return (
              <label
                key={student.id}
                aria-label={`${student.first_name} ${student.second_name} auswählen`}
                className="flex cursor-pointer items-center gap-3 rounded-xl border border-gray-200 bg-white px-3 py-2 transition-colors hover:bg-gray-50"
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => toggleSelected(studentId)}
                  className="h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-base font-semibold text-gray-900">
                    {student.first_name} {student.second_name}
                  </p>
                  <p className="mt-0.5 truncate text-sm text-gray-500">
                    {[student.school_class, student.group_name]
                      .filter(Boolean)
                      .join(" · ")}
                  </p>
                </div>
              </label>
            );
          })
        )}
      </div>
    </section>
  );
}
