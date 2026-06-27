"use client";

import { MapPin, Search, UserPlus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { fetchStudents } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import type { StudentMoveResult } from "~/lib/active-service";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentMovePanel" });

interface StudentMovePanelProps {
  readonly targetActiveGroupId: string;
  readonly targetLabel: string;
  readonly currentStudentIds: readonly string[];
  readonly onMove: (studentIds: string[]) => Promise<StudentMoveResult>;
}

function studentDisplayName(student: Student): string {
  return (
    student.name ||
    [student.first_name, student.second_name].filter(Boolean).join(" ") ||
    `Kind ${student.id}`
  );
}

function studentMeta(student: Student): string {
  return [student.school_class, student.group_name].filter(Boolean).join(" · ");
}

function locationLabel(location?: string | null): string {
  if (!location) return "Unterwegs";
  const withoutPresencePrefix = location.replace(/^Anwesend\s*[-–]\s*/i, "");
  if (!withoutPresencePrefix.trim()) return "Unterwegs";
  return withoutPresencePrefix;
}

function skippedMessage(result: StudentMoveResult): string | null {
  if (result.skipped.length === 0) return null;
  const notPresent = result.skipped.filter(
    (item) => item.reason === "not_present",
  ).length;
  if (notPresent === result.skipped.length) {
    return notPresent === 1
      ? "Kind ist noch nicht anwesend."
      : `${notPresent} Kinder sind noch nicht anwesend.`;
  }
  return `${result.skipped.length} ${
    result.skipped.length === 1 ? "Kind konnte" : "Kinder konnten"
  } nicht bewegt werden.`;
}

export function StudentMovePanel({
  targetActiveGroupId,
  targetLabel,
  currentStudentIds,
  onMove,
}: StudentMovePanelProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Student[]>([]);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const currentStudentIdSet = useMemo(
    () => new Set(currentStudentIds.map(String)),
    [currentStudentIds],
  );
  const selectedMovableIds = useMemo(
    () =>
      [...selectedIds].filter(
        (studentId) => !currentStudentIdSet.has(studentId),
      ),
    [currentStudentIdSet, selectedIds],
  );

  useEffect(() => {
    const trimmed = query.trim();
    setMessage(null);
    setError(null);

    if (trimmed.length < 2) {
      setResults([]);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    const timeout = window.setTimeout(() => {
      fetchStudents({
        search: trimmed,
        location_state: "present",
        page: 1,
        page_size: 10,
      })
        .then((response) => {
          setResults(response.students);
        })
        .catch((err: unknown) => {
          logger.warn("active_supervision_move_search_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          setResults([]);
          setError("Kinder konnten nicht gesucht werden.");
        })
        .finally(() => setIsLoading(false));
    }, 250);

    return () => window.clearTimeout(timeout);
  }, [query]);

  useEffect(() => {
    setSelectedIds((current) => {
      const next = new Set(current);
      for (const id of next) {
        if (currentStudentIdSet.has(id)) next.delete(id);
      }
      return next;
    });
  }, [currentStudentIdSet]);

  function toggleStudent(studentId: string) {
    if (currentStudentIdSet.has(studentId)) return;
    setSelectedIds((current) => {
      const next = new Set(current);
      if (next.has(studentId)) next.delete(studentId);
      else next.add(studentId);
      return next;
    });
    setMessage(null);
    setError(null);
  }

  async function submitMove() {
    if (selectedMovableIds.length === 0) {
      setError("Bitte mindestens ein Kind auswählen.");
      return;
    }

    try {
      setIsSubmitting(true);
      setError(null);
      setMessage(null);
      const result = await onMove(selectedMovableIds);
      const skipped = skippedMessage(result);
      if (skipped) {
        setError(skipped);
      } else {
        const count = result.moved.length + result.unchanged.length;
        setMessage(
          `${count} ${count === 1 ? "Kind" : "Kinder"} nach ${targetLabel} bewegt.`,
        );
        setQuery("");
        setResults([]);
      }
      setSelectedIds(new Set());
    } catch (err) {
      logger.warn("active_supervision_move_failed", {
        target_active_group_id: targetActiveGroupId,
        count: selectedMovableIds.length,
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Ausgewählte Kinder konnten nicht bewegt werden.");
    } finally {
      setIsSubmitting(false);
    }
  }

  const selectableResultCount = results.filter(
    (student) => !currentStudentIdSet.has(student.id.toString()),
  ).length;

  return (
    <section className="mb-4 rounded-lg border border-gray-200 bg-white p-3 shadow-sm sm:p-4">
      <div className="mb-3 flex items-center gap-2">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-700">
          <UserPlus className="h-4 w-4" aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-gray-900">
            Kind nach {targetLabel} bewegen
          </h2>
          <p className="text-xs text-gray-500">
            Nur heute anwesende Kinder können bewegt werden.
          </p>
        </div>
      </div>

      {error ? <Alert type="error" message={error} /> : null}
      {message ? <Alert type="success" message={message} /> : null}

      <div className="mt-3">
        <div className="relative">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 z-10 h-4 w-4 -translate-y-1/2 text-gray-400"
            aria-hidden="true"
          />
          <Input
            type="search"
            name="active-supervision-student-move-search"
            controlSize="compact"
            className="pl-9"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Kind suchen..."
            autoComplete="off"
          />
        </div>
      </div>

      {query.trim().length > 0 && query.trim().length < 2 ? (
        <p className="mt-2 text-xs text-gray-500">
          Bitte mindestens 2 Zeichen eingeben.
        </p>
      ) : null}

      {results.length > 0 ? (
        <div className="mt-3 divide-y divide-gray-100 overflow-hidden rounded-lg border border-gray-200">
          {results.map((student) => {
            const studentId = student.id.toString();
            const checkboxId = `active-supervision-move-student-${studentId}`;
            const alreadyHere = currentStudentIdSet.has(studentId);
            const checked = selectedIds.has(studentId);
            return (
              <label
                key={studentId}
                htmlFor={checkboxId}
                className={`flex min-h-16 items-center gap-3 px-3 py-2 ${
                  alreadyHere
                    ? "cursor-not-allowed bg-gray-50 text-gray-400"
                    : "cursor-pointer bg-white hover:bg-gray-50"
                }`}
              >
                <Checkbox
                  id={checkboxId}
                  checked={checked}
                  disabled={alreadyHere || isSubmitting}
                  onChange={() => toggleStudent(studentId)}
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-gray-900">
                    {studentDisplayName(student)}
                  </span>
                  <span className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500">
                    {studentMeta(student) ? (
                      <span>{studentMeta(student)}</span>
                    ) : null}
                    <span className="inline-flex items-center gap-1">
                      <MapPin className="h-3 w-3" aria-hidden="true" />
                      {alreadyHere
                        ? "Bereits hier"
                        : locationLabel(student.current_location)}
                    </span>
                  </span>
                </span>
              </label>
            );
          })}
        </div>
      ) : null}

      {query.trim().length >= 2 && !isLoading && results.length === 0 ? (
        <p className="mt-3 rounded-lg bg-gray-50 px-3 py-2 text-sm text-gray-500">
          Keine anwesenden Kinder gefunden.
        </p>
      ) : null}

      <div className="mt-3 flex items-center justify-between gap-3">
        <span className="text-xs text-gray-500">
          {selectableResultCount > 0
            ? `${selectedMovableIds.length} ausgewählt`
            : "Keine auswählbaren Treffer"}
        </span>
        <Button
          type="button"
          variant="primary"
          size="md"
          isLoading={isSubmitting}
          loadingText="Bewegt ..."
          disabled={isSubmitting || selectedMovableIds.length === 0}
          onClick={() => void submitMove()}
        >
          Bewegen
        </Button>
      </div>
    </section>
  );
}
