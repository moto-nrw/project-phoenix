"use client";

import { useCallback, useEffect, useState } from "react";
import { Footprints, Loader2, Plus, Save, Search, X } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { fetchStudents } from "~/lib/student-api";
import {
  ALL_COMPANION_WEEKDAYS,
  COMPANION_WEEKDAYS,
  companionDisplayName,
  fetchStudentCompanions,
  updateStudentCompanions,
  type CompanionWeekday,
  type StudentCompanion,
} from "~/lib/student-companion-api";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CompanionSection" });

// Mirrors the guardian picker: short queries would turn an all-staff control
// into a way to page through the whole child roster one letter at a time.
const MIN_QUERY_LENGTH = 2;
const SEARCH_DEBOUNCE_MS = 300;
const SEARCH_RESULT_LIMIT = 8;

interface SearchResult {
  id: number;
  name: string;
  schoolClass: string;
}

/**
 * "Läuft mit" — the children this child walks home with (Laufgemeinschaft).
 *
 * Self-contained: it loads and saves through its own endpoint rather than the
 * surrounding Stammdaten form, because the link is stored on its own table and
 * writing it also changes what the OTHER child's card shows. Keeping it out of
 * the big form means a companion edit can never be half-applied by an unrelated
 * validation error elsewhere in the form.
 */
export default function CompanionSection({
  studentId,
  canEdit = true,
  variant = "section",
}: Readonly<{
  studentId: string;
  /** False renders the list read-only (no picker, no save). */
  canEdit?: boolean;
  /**
   * "section" sits inside the Stammdaten edit form and matches the other form
   * blocks; "card" is the standalone surface used on the child detail page,
   * where it has to look like the cards around it.
   */
  variant?: "section" | "card";
}>) {
  const [companions, setCompanions] = useState<StudentCompanion[]>([]);
  const [saved, setSaved] = useState<StudentCompanion[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [pickerOpen, setPickerOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchStudentCompanions(studentId)
      .then((loaded) => {
        if (cancelled) return;
        setCompanions(loaded);
        setSaved(loaded);
        setError(null);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("failed to load companions", { error: err });
        setError("Laufgemeinschaft konnte nicht geladen werden.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [studentId]);

  // Debounced child search. The current child and everyone already linked are
  // filtered out client-side so the list only offers valid additions.
  useEffect(() => {
    if (!pickerOpen) return;
    const trimmed = query.trim();
    if (trimmed.length < MIN_QUERY_LENGTH) {
      setResults([]);
      return;
    }

    let cancelled = false;
    setSearching(true);
    const timer = setTimeout(() => {
      fetchStudents({ search: trimmed, page_size: 25 })
        .then(({ students }) => {
          if (cancelled) return;
          const linked = new Set(
            companions.map((companion) => companion.companion_student_id),
          );
          setResults(
            students
              .filter(
                (student) =>
                  String(student.id) !== String(studentId) &&
                  !linked.has(Number(student.id)),
              )
              .slice(0, SEARCH_RESULT_LIMIT)
              .map((student) => ({
                id: Number(student.id),
                // `name` is the derived "Vorname Nachname" the list already
                // ships; first_name/second_name are only sometimes populated.
                name:
                  student.name?.trim() ||
                  `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim(),
                schoolClass: student.school_class ?? "",
              })),
          );
        })
        .catch((err: unknown) => {
          if (cancelled) return;
          logger.error("child search failed", { error: err });
          setResults([]);
        })
        .finally(() => {
          if (!cancelled) setSearching(false);
        });
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [query, pickerOpen, companions, studentId]);

  const isDirty = !sameCompanions(companions, saved);

  const addCompanion = useCallback((result: SearchResult) => {
    // New links default to the whole week — the common case is a child that
    // always walks with the same neighbour, and unticking days is quicker than
    // ticking five.
    setCompanions((prev) => [
      ...prev,
      {
        companion_student_id: result.id,
        first_name: result.name.split(" ")[0] ?? result.name,
        last_name: result.name.split(" ").slice(1).join(" "),
        weekdays: [...ALL_COMPANION_WEEKDAYS],
      },
    ]);
    setPickerOpen(false);
    setQuery("");
    setResults([]);
  }, []);

  const toggleWeekday = useCallback(
    (companionId: number, weekday: CompanionWeekday) => {
      setCompanions((prev) =>
        prev.map((companion) => {
          if (companion.companion_student_id !== companionId) return companion;
          const next = companion.weekdays.includes(weekday)
            ? companion.weekdays.filter((day) => day !== weekday)
            : [...companion.weekdays, weekday];
          return {
            ...companion,
            weekdays: ALL_COMPANION_WEEKDAYS.filter((day) =>
              next.includes(day),
            ),
          };
        }),
      );
    },
    [],
  );

  const removeCompanion = useCallback((companionId: number) => {
    setCompanions((prev) =>
      prev.filter(
        (companion) => companion.companion_student_id !== companionId,
      ),
    );
  }, []);

  const handleSave = useCallback(async () => {
    // A link with no weekday applies on no day — the backend rejects it, so
    // catch it here with a message that points at the offending row.
    const empty = companions.find(
      (companion) => companion.weekdays.length === 0,
    );
    if (empty) {
      setError(
        `Bitte mindestens einen Wochentag für ${companionDisplayName(empty)} auswählen.`,
      );
      return;
    }

    setSaving(true);
    setError(null);
    try {
      const updated = await updateStudentCompanions(
        studentId,
        companions.map((companion) => ({
          companion_student_id: companion.companion_student_id,
          weekdays: companion.weekdays,
        })),
      );
      setCompanions(updated);
      setSaved(updated);
    } catch (err: unknown) {
      logger.error("failed to save companions", { error: err });
      setError(
        err instanceof Error
          ? err.message
          : "Laufgemeinschaft konnte nicht gespeichert werden.",
      );
    } finally {
      setSaving(false);
    }
  }, [companions, studentId]);

  const isCard = variant === "card";

  return (
    <div
      className={
        isCard
          ? "moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6"
          : "rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4"
      }
    >
      <div className="mb-3 flex items-start justify-between gap-3 md:mb-4">
        <h3
          className={
            isCard
              ? "flex min-w-0 items-center gap-2 text-base font-semibold text-gray-900 sm:text-lg"
              : "flex min-w-0 items-center gap-2 text-xs font-semibold text-gray-900 md:text-sm"
          }
        >
          <Footprints
            className={
              isCard ? "h-5 w-5 shrink-0" : "h-3.5 w-3.5 shrink-0 md:h-4 md:w-4"
            }
            style={{ color: GROUP_ROOM_SHADES.text }}
            aria-hidden
          />
          Läuft mit
        </h3>
        {companions.length > 0 && (
          <span className="hidden shrink-0 rounded-full bg-[#DCF5C1] px-2.5 py-1 text-xs font-medium text-[#4a7a15] sm:inline">
            {companions.length === 1 ? "1 Kind" : `${companions.length} Kinder`}
          </span>
        )}
      </div>
      <p className="mb-3 text-xs text-gray-500">
        Kinder, mit denen dieses Kind nach Hause geht. Die Verknüpfung gilt für
        beide Kinder.
      </p>

      {loading ? (
        <div className="flex items-center gap-2 text-xs text-gray-500">
          <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />
          Wird geladen...
        </div>
      ) : (
        <div className="space-y-2">
          {companions.length === 0 && !pickerOpen ? (
            <p className="text-xs text-gray-500">
              Noch keine Verknüpfung hinterlegt.
            </p>
          ) : null}

          {companions.map((companion) => (
            <div
              key={companion.companion_student_id}
              className="rounded-lg border border-gray-200 bg-white p-2.5"
            >
              <div className="mb-2 flex items-center justify-between gap-2">
                <span className="truncate text-sm font-medium text-gray-900">
                  {companionDisplayName(companion)}
                </span>
                {canEdit && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={`${companionDisplayName(companion)} entfernen`}
                    onClick={() =>
                      removeCompanion(companion.companion_student_id)
                    }
                  >
                    <X className="h-4 w-4" aria-hidden />
                  </Button>
                )}
              </div>
              {/* Capped: on the wide detail-page card five chips would
                  otherwise stretch across the full width and read as buttons
                  rather than a compact weekday picker. */}
              <div className="grid max-w-sm grid-cols-5 gap-1">
                {COMPANION_WEEKDAYS.map((day) => {
                  const active = companion.weekdays.includes(day.value);
                  return (
                    <label
                      key={day.value}
                      className={`flex h-8 items-center justify-center gap-1.5 rounded-lg border px-1 text-xs font-semibold transition-colors ${
                        canEdit ? "cursor-pointer" : "cursor-default"
                      } ${
                        active
                          ? "border-gray-900 bg-white text-gray-900"
                          : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
                      }`}
                    >
                      <Checkbox
                        checked={active}
                        disabled={!canEdit}
                        aria-label={`${companionDisplayName(companion)}: ${day.label}`}
                        onChange={() =>
                          toggleWeekday(
                            companion.companion_student_id,
                            day.value,
                          )
                        }
                      />
                      {day.short}
                    </label>
                  );
                })}
              </div>
            </div>
          ))}

          {canEdit && pickerOpen ? (
            <div className="rounded-lg border border-gray-200 bg-white p-2.5">
              <div className="relative mb-2">
                <Search
                  className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400"
                  aria-hidden
                />
                <input
                  type="text"
                  autoFocus
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Kind suchen (Name)"
                  aria-label="Kind für die Laufgemeinschaft suchen"
                  className="block w-full rounded-lg border border-gray-200 py-2 pr-3 pl-9 text-sm transition-colors focus:border-gray-400 focus:ring-1 focus:ring-gray-400"
                />
              </div>
              {query.trim().length < MIN_QUERY_LENGTH ? (
                <p className="text-xs text-gray-500">
                  Mindestens {MIN_QUERY_LENGTH} Zeichen eingeben.
                </p>
              ) : searching ? (
                <div className="flex items-center gap-2 text-xs text-gray-500">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />
                  Suche...
                </div>
              ) : results.length === 0 ? (
                <p className="text-xs text-gray-500">Kein Kind gefunden.</p>
              ) : (
                <ul className="space-y-1">
                  {results.map((result) => (
                    <li key={result.id}>
                      <button
                        type="button"
                        onClick={() => addCompanion(result)}
                        className="flex w-full items-center justify-between gap-2 rounded-lg border border-gray-200 px-2.5 py-2 text-left text-sm text-gray-900 transition-colors hover:border-gray-300 hover:bg-gray-50"
                      >
                        <span className="truncate">{result.name}</span>
                        {result.schoolClass ? (
                          <span className="shrink-0 text-xs text-gray-500">
                            {result.schoolClass}
                          </span>
                        ) : null}
                      </button>
                    </li>
                  ))}
                </ul>
              )}
              <div className="mt-2 flex justify-end">
                <Button
                  type="button"
                  variant="ghost"
                  size="md"
                  onClick={() => {
                    setPickerOpen(false);
                    setQuery("");
                    setResults([]);
                  }}
                >
                  Abbrechen
                </Button>
              </div>
            </div>
          ) : null}

          {error ? (
            <p className="text-xs font-medium text-[#FF3130]">{error}</p>
          ) : null}

          {canEdit && !pickerOpen ? (
            <div className="flex items-center justify-between gap-2 pt-1">
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => setPickerOpen(true)}
              >
                <Plus className="mr-1.5 h-4 w-4" aria-hidden />
                Kind hinzufügen
              </Button>
              {isDirty ? (
                <Button
                  type="button"
                  variant="primary"
                  size="md"
                  disabled={saving}
                  onClick={() => void handleSave()}
                >
                  {saving ? (
                    <>
                      <Loader2
                        className="mr-1.5 h-4 w-4 animate-spin"
                        aria-hidden
                      />
                      Speichern...
                    </>
                  ) : (
                    <>
                      <Save className="mr-1.5 h-4 w-4" aria-hidden />
                      Laufgemeinschaft speichern
                    </>
                  )}
                </Button>
              ) : null}
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}

/** Order-insensitive comparison so re-ordered but identical lists stay clean. */
function sameCompanions(
  a: readonly StudentCompanion[],
  b: readonly StudentCompanion[],
): boolean {
  if (a.length !== b.length) return false;
  const key = (list: readonly StudentCompanion[]) =>
    list
      .map(
        (companion) =>
          `${companion.companion_student_id}:${[...companion.weekdays].sort().join("")}`,
      )
      .sort()
      .join("|");
  return key(a) === key(b);
}
