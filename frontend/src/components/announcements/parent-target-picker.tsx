"use client";

import { groups as groupsKeys } from "~/lib/swr/groups";
import { useEffect, useRef, useState } from "react";
import { Check } from "lucide-react";

import { summarizeTargets } from "~/components/announcements/announcement-meta";
import type { FormError } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { studentService } from "~/lib/api";
import { fetchActivities } from "~/lib/activity-api";
import type { Group, Student } from "~/lib/api";
import type { Activity } from "~/lib/activity-helpers";
import type { AnnouncementTarget } from "~/lib/parent-announcements-api";

/*
 * Die Empfängerwahl für Eltern: ganze Schule, Klassen, Gruppen, AGs und
 * einzelne Kinder. Geteilt von den Elternmitteilungen und den Umfragen
 * (#4781), damit beide dieselbe Auswahl mit denselben Wörtern zeigen.
 */

/** Stable identity for a target so duplicates are rejected and React keys are unique. */
function targetKey(target: AnnouncementTarget): string {
  return `${target.target_type}:${target.ref_id ?? target.ref_text ?? ""}`;
}

/**
 * Broad-audience toggle pill (mirrors the operator announcement modal's
 * selector): a bordered button with an inline check-box.
 */
function CheckPill({
  label,
  active,
  onClick,
}: {
  readonly label: string;
  readonly active: boolean;
  readonly onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex w-full min-w-0 items-center gap-2 rounded-lg border px-3 py-2 text-left text-sm transition-all ${
        active
          ? "border-gray-900 bg-gray-50 text-gray-900"
          : "border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50"
      }`}
    >
      <span
        className={`flex h-4 w-4 shrink-0 items-center justify-center rounded border transition-all ${
          active ? "border-gray-900 bg-gray-900" : "border-gray-300 bg-white"
        }`}
      >
        {active && <Check className="h-3 w-3 text-white" strokeWidth={3} />}
      </span>
      <span className="truncate">{label}</span>
    </button>
  );
}

export interface ParentTargetPickerProps {
  readonly targets: AnnouncementTarget[];
  readonly groups: Group[];
  readonly activities: Activity[];
  readonly schoolClasses: string[];
  readonly studentNames: Record<string, string>;
  readonly onChange: (targets: AnnouncementTarget[]) => void;
  readonly onSetStudentName: (id: string, name: string) => void;
  /** Names the thing being addressed in the heading (Mitteilung vs Umfrage). */
  readonly kindLabel: string;
  /**
   * Replaces the whole heading where "Wer soll … erhalten?" reads wrong: an
   * Einverständnis is asked for, not received.
   */
  readonly heading?: string;
  /**
   * Offene Anmeldungen reach applicants who have no enrolled child yet — so a
   * poll would show them a question they cannot answer (answers are per child).
   * Polls therefore hide the option; the backend refuses it as well.
   */
  readonly allowPendingEnrollment: boolean;
  /** Gruppen, AGs oder Klassen konnten nicht geladen werden. */
  readonly lookupLoadError: FormError | null;
}

export function ParentTargetPicker({
  targets,
  groups,
  activities,
  schoolClasses,
  studentNames,
  onChange,
  onSetStudentName,
  kindLabel,
  heading,
  allowPendingEnrollment,
  lookupLoadError,
}: ParentTargetPickerProps) {
  // Single source of truth is `targets`; each control derives its selection
  // from it and rebuilds it on change.
  const schoolAll = targets.some((t) => t.target_type === "school_all");
  const pendingEnrollment = targets.some(
    (t) => t.target_type === "pending_enrollment",
  );
  const selectedClasses = targets
    .filter((t) => t.target_type === "class")
    .map((t) => t.ref_text ?? "");
  const selectedGroups = targets
    .filter((t) => t.target_type === "group")
    .map((t) => t.ref_id ?? "");
  const selectedActivities = targets
    .filter((t) => t.target_type === "activity_group")
    .map((t) => t.ref_id ?? "");
  const selectedStudents = targets.filter((t) => t.target_type === "student");

  const toggleSimple = (
    targetType: "school_all" | "pending_enrollment",
    on: boolean,
  ) => {
    const without = targets.filter((t) => t.target_type !== targetType);
    onChange(on ? [...without, { target_type: targetType }] : without);
  };

  const setCategory = (
    targetType: "class" | "group" | "activity_group",
    values: string[],
  ) => {
    const others = targets.filter((t) => t.target_type !== targetType);
    const next = values.map<AnnouncementTarget>((v) =>
      targetType === "class"
        ? { target_type: "class", ref_text: v }
        : { target_type: targetType, ref_id: v },
    );
    onChange([...others, ...next]);
  };

  const addStudent = (student: Student) => {
    onSetStudentName(student.id, student.name);
    if (selectedStudents.some((t) => t.ref_id === student.id)) return;
    onChange([...targets, { target_type: "student", ref_id: student.id }]);
  };

  const removeStudent = (id: string) => {
    onChange(
      targets.filter((t) => !(t.target_type === "student" && t.ref_id === id)),
    );
  };

  // Students are too many to list, so this stays a debounced search; the chosen
  // children render as removable chips below it.
  const [studentSearch, setStudentSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const searchTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (searchTimeout.current) clearTimeout(searchTimeout.current);
    searchTimeout.current = setTimeout(
      () => setDebouncedSearch(studentSearch.trim()),
      300,
    );
    return () => {
      if (searchTimeout.current) clearTimeout(searchTimeout.current);
    };
  }, [studentSearch]);

  const {
    data: studentResults,
    isLoading: studentsLoading,
    error: studentsError,
    mutate: retryStudentSearch,
  } = useSWRAuth<Student[]>(
    debouncedSearch.length >= 2
      ? `parent-announcements-student-search-${debouncedSearch}`
      : null,
    async () => {
      const result = await studentService.getStudents({
        search: debouncedSearch,
        pageSize: 20,
      });
      return result.students;
    },
    { revalidateOnFocus: false },
  );
  // Eine gescheiterte Suche ist kein „Keine Kinder gefunden“.
  const studentsLoadError = useSwrLoadError(
    studentsError,
    "die Suche nach Kindern",
    () => void retryStudentSearch(),
  );

  const fieldLabel = "mb-1.5 block text-sm font-medium text-gray-700";

  return (
    <div className="space-y-4">
      <div>
        <h4 className="text-sm font-semibold text-gray-900">
          {heading ?? `Wer soll ${kindLabel} erhalten?`}
        </h4>
        <p className="mt-0.5 text-xs text-gray-500">
          Sie können mehrere Zielgruppen wählen. Jedes Elternteil erhält{" "}
          {kindLabel} höchstens einmal.
        </p>
      </div>

      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        <CheckPill
          label="Ganze Schule"
          active={schoolAll}
          onClick={() => toggleSimple("school_all", !schoolAll)}
        />
        {allowPendingEnrollment && (
          <CheckPill
            label="Offene Anmeldungen"
            active={pendingEnrollment}
            onClick={() =>
              toggleSimple("pending_enrollment", !pendingEnrollment)
            }
          />
        )}
      </div>

      {!schoolAll && (
        <>
          <LoadErrorAlert error={lookupLoadError} />
          <div className="grid gap-3 sm:grid-cols-3">
            <div>
              <span className={fieldLabel}>Klassen</span>
              <MultiCheckboxSelect
                ariaLabel="Klassen auswählen"
                value={selectedClasses}
                options={schoolClasses.map((c) => ({ value: c, label: c }))}
                onChange={(values) => setCategory("class", values)}
                emptyLabel="Keine ausgewählt"
                unavailableLabel="Keine Klassen"
                searchable
                searchPlaceholder="Klasse suchen…"
              />
            </div>
            <div>
              <span className={fieldLabel}>Gruppen</span>
              <MultiCheckboxSelect
                ariaLabel="Gruppen auswählen"
                value={selectedGroups}
                options={groups.map((g) => ({ value: g.id, label: g.name }))}
                onChange={(values) => setCategory("group", values)}
                emptyLabel="Keine ausgewählt"
                unavailableLabel="Keine Gruppen"
                searchable
                searchPlaceholder="Gruppe suchen…"
              />
            </div>
            <div>
              <span className={fieldLabel}>AGs / Betreuung</span>
              <MultiCheckboxSelect
                ariaLabel="AGs auswählen"
                value={selectedActivities}
                options={activities.map((a) => ({
                  value: a.id,
                  label: a.name,
                }))}
                onChange={(values) => setCategory("activity_group", values)}
                emptyLabel="Keine ausgewählt"
                unavailableLabel="Keine AGs"
                searchable
                searchPlaceholder="AG suchen…"
              />
            </div>
          </div>

          <div>
            <span className={fieldLabel}>Einzelne Kinder</span>
            {selectedStudents.length > 0 && (
              <div className="mb-2 flex flex-wrap gap-2">
                {selectedStudents.map((t) => (
                  <button
                    key={targetKey(t)}
                    type="button"
                    onClick={() => t.ref_id && removeStudent(t.ref_id)}
                    title="Entfernen"
                    className="inline-flex items-center gap-1.5 rounded-full border border-gray-300 bg-gray-50 px-3 py-1 text-sm text-gray-800 transition-colors hover:border-gray-400 hover:bg-gray-100"
                  >
                    <span className="max-w-48 truncate">
                      {t.ref_id ? (studentNames[t.ref_id] ?? "Kind") : "Kind"}
                    </span>
                    <span aria-hidden className="text-gray-400">
                      ×
                    </span>
                  </button>
                ))}
              </div>
            )}
            <Input
              name="announcement-student-search"
              controlSize="compact"
              value={studentSearch}
              onChange={(e) => setStudentSearch(e.target.value)}
              placeholder="Kind suchen (mind. 2 Zeichen)…"
            />
            {debouncedSearch.length >= 2 && (
              <div className="mt-2 max-h-40 overflow-y-auto rounded-lg border border-gray-200">
                {studentsLoadError ? (
                  <LoadErrorAlert error={studentsLoadError} />
                ) : studentsLoading || (studentsError && !studentResults) ? (
                  <p className="px-3 py-2 text-sm text-gray-500">
                    Wird gesucht…
                  </p>
                ) : (studentResults?.length ?? 0) === 0 ? (
                  <p className="px-3 py-2 text-sm text-gray-500">
                    Keine Kinder gefunden.
                  </p>
                ) : (
                  studentResults?.map((student) => (
                    <button
                      key={student.id}
                      type="button"
                      onClick={() => addStudent(student)}
                      className="flex w-full items-center justify-between px-3 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
                    >
                      <span className="truncate">{student.name}</span>
                      {student.school_class && (
                        <span className="ml-2 shrink-0 text-xs text-gray-400">
                          {student.school_class}
                        </span>
                      )}
                    </button>
                  ))
                )}
              </div>
            )}
          </div>
        </>
      )}

      <p className="rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-700">
        Ausgewählt:{" "}
        <span className="font-medium">{summarizeTargets(targets)}</span>
      </p>
    </div>
  );
}

/**
 * Die Nachschlagelisten der Empfängerwahl: Gruppen, AGs und Klassen. Sie
 * laden erst, wenn die Auswahl sichtbar ist. Ohne geladene Liste stünde in
 * den Feldern „Keine Gruppen“, obwohl es welche gibt; der Fehler steht
 * deshalb über der Auswahl.
 */
export function useParentTargetLookups(enabled: boolean): {
  groups: Group[];
  activities: Activity[];
  schoolClasses: string[];
  lookupLoadError: FormError | null;
} {
  const {
    data: groups,
    error: groupsError,
    mutate: reloadGroups,
  } = useSWRAuth<Group[]>(
    enabled ? groupsKeys.key() : null,
    groupsKeys.fetcher,
    { revalidateOnFocus: false },
  );
  const {
    data: activities,
    error: activitiesError,
    mutate: reloadActivities,
  } = useSWRAuth<Activity[]>(
    enabled ? "parent-announcements-activities" : null,
    () => fetchActivities(),
    { revalidateOnFocus: false },
  );
  const {
    data: schoolClasses,
    error: classesError,
    mutate: reloadClasses,
  } = useSWRAuth<string[]>(
    enabled ? "parent-announcements-classes" : null,
    () => studentService.getSchoolClasses(),
    { revalidateOnFocus: false },
  );
  const groupsLoadError = useSwrLoadError(
    groupsError,
    "die Liste der Gruppen",
    () => void reloadGroups(),
  );
  const activitiesLoadError = useSwrLoadError(
    activitiesError,
    "die Liste der AGs",
    () => void reloadActivities(),
  );
  const classesLoadError = useSwrLoadError(
    classesError,
    "die Liste der Klassen",
    () => void reloadClasses(),
  );
  return {
    groups: groups ?? [],
    activities: activities ?? [],
    schoolClasses: schoolClasses ?? [],
    lookupLoadError: groupsLoadError ?? activitiesLoadError ?? classesLoadError,
  };
}
