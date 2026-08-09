"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Loader2, Plus, Search, X } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { fetchStudents } from "~/lib/student-api";
import {
  allowedModesIncludeAccompanied,
  DEPARTURE_WEEKDAYS,
  type AllowedDepartureModes,
} from "~/lib/student-helpers";
import {
  COMPANION_WEEKDAYS,
  companionDisplayName,
  type CompanionExtensionConfirmation,
  type CompanionWeekday,
  type StudentCompanion,
} from "~/lib/student-companion-api";
import { getGermanWeekdayAdverb } from "~/lib/timetable-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CompanionPicker" });

// Mirrors the guardian picker: short queries would turn an all-staff control
// into a way to page through the whole child roster one letter at a time.
const MIN_QUERY_LENGTH = 2;
const SEARCH_DEBOUNCE_MS = 300;
const SEARCH_RESULT_LIMIT = 8;

interface SearchResult {
  /** Backend int64 id, carried as a string like every other frontend id. */
  id: string;
  name: string;
  schoolClass: string;
  /** The candidate's own plan, used to spot a mismatch before saving. */
  allowedDepartureModes?: AllowedDepartureModes;
}

/**
 * "Läuft mit": the children this child walks home with (Laufgemeinschaft).
 *
 * Controlled on purpose. The link is only legal on a weekday the child's
 * departure plan allows "Anderes Kind", so it cannot be edited or saved
 * independently of that plan — the host form owns both and submits them
 * together.
 */
export function CompanionPicker({
  value,
  onChange,
  allowedDays,
  excludeStudentId,
  onExtensionConfirmed,
  disabled = false,
}: Readonly<{
  value: StudentCompanion[];
  onChange: (next: StudentCompanion[]) => void;
  /** Weekdays the child may leave with another child, from the plan above. */
  allowedDays: CompanionWeekday[];
  /** The child being edited, filtered out of the search results. */
  excludeStudentId?: string;
  /**
   * Called when the user confirmed widening a companion's own departure plan,
   * with the child and the weekdays the question named. The host sends both
   * along: the backend only widens what the confirmation actually covers.
   */
  onExtensionConfirmed?: (confirmation: CompanionExtensionConfirmation) => void;
  disabled?: boolean;
}>) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [pending, setPending] = useState<{
    result: SearchResult;
    missingDays: CompanionWeekday[];
  } | null>(null);

  const dayOptions = COMPANION_WEEKDAYS.filter((day) =>
    allowedDays.includes(day.value),
  );
  // Content identity of the plan's accompanied days: the prop is a fresh array
  // on every render, so only this tells a real plan change from a re-render.
  const allowedKey = dayOptions.map((day) => day.value).join("|");
  const lastAllowedKeyRef = useRef(allowedKey);

  // Drop weekdays that a PLAN EDIT just took away, so unticking "Anderes Kind"
  // for Thursday cannot leave a Thursday link behind. Entries left without any
  // day disappear entirely — a link on no day is not a link.
  //
  // Bound to a CHANGE of the allowed days on purpose, never to `value`: the
  // list is also replaced wholesale when the stored links are re-fetched after
  // a remote write, and that list is current while the plan above can still be
  // the pre-write one the page has not revalidated yet. Trimming the fresh list
  // against the stale plan would delete a link somebody else just created — and
  // since the save then carries the fingerprint of exactly that freshly loaded
  // list, the backend's baseline check would see a matching baseline and accept
  // the deletion instead of refusing it. An untouched plan removes nothing
  // here; a narrowed plan that is actually saved is trimmed by the backend,
  // against the baseline it was given.
  useEffect(() => {
    if (lastAllowedKeyRef.current === allowedKey) return;
    lastAllowedKeyRef.current = allowedKey;
    const trimmed = value
      .map((companion) => ({
        ...companion,
        weekdays: companion.weekdays.filter((day) => allowedDays.includes(day)),
      }))
      .filter((companion) => companion.weekdays.length > 0);
    const changed =
      trimmed.length !== value.length ||
      trimmed.some(
        (companion, index) =>
          companion.weekdays.length !== value[index]?.weekdays.length,
      );
    if (changed) onChange(trimmed);
  }, [allowedKey, allowedDays, value, onChange]);

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
            value.map((companion) => companion.companion_student_id),
          );
          setResults(
            students
              .filter(
                (student) =>
                  String(student.id) !== String(excludeStudentId) &&
                  !linked.has(String(student.id)),
              )
              .slice(0, SEARCH_RESULT_LIMIT)
              .map((student) => ({
                id: String(student.id),
                name:
                  student.name?.trim() ||
                  `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim(),
                schoolClass: student.school_class ?? "",
                allowedDepartureModes: student.allowed_departure_modes,
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
  }, [query, pickerOpen, value, excludeStudentId]);

  const closePicker = useCallback(() => {
    setPickerOpen(false);
    setQuery("");
    setResults([]);
    setPending(null);
  }, []);

  const commitCompanion = useCallback(
    (result: SearchResult) => {
      onChange([
        ...value,
        {
          companion_student_id: result.id,
          first_name: result.name.split(" ")[0] ?? result.name,
          last_name: result.name.split(" ").slice(1).join(" "),
          weekdays: [...allowedDays],
        },
      ]);
      closePicker();
    },
    [value, allowedDays, onChange, closePicker],
  );

  const selectCompanion = useCallback(
    (result: SearchResult) => {
      // The other child needs "Anderes Kind" on the same days, otherwise their
      // own Stammdaten would contradict the link. Ask BEFORE saving rather than
      // letting the backend reject it — the answer is a one-click yes, and
      // widening another child's departure permission must be deliberate.
      const missingDays = missingAccompaniedDays(
        result.allowedDepartureModes,
        allowedDays,
      );
      if (missingDays.length > 0) {
        setPending({ result, missingDays });
        return;
      }
      commitCompanion(result);
    },
    [allowedDays, commitCompanion],
  );

  const toggleWeekday = useCallback(
    (companionId: string, weekday: CompanionWeekday) => {
      onChange(
        value.flatMap((companion) => {
          if (companion.companion_student_id !== companionId)
            return [companion];
          const next = companion.weekdays.includes(weekday)
            ? companion.weekdays.filter((day) => day !== weekday)
            : [...companion.weekdays, weekday];
          // Mo..Fr order, and deliberately not filtered to allowedDays: a day
          // the plan above does not currently allow has no checkbox in this
          // row, so ticking another one is no statement about it. A list loaded
          // after a remote write can carry such a day while the plan prop is
          // still the pre-write one — dropping it here would delete somebody
          // else's day on the back of an unrelated tick.
          const weekdays = COMPANION_WEEKDAYS.filter((day) =>
            next.includes(day.value),
          ).map((day) => day.value);
          // Unticking the last day is how the user says "not this child after
          // all". Keeping the entry with an empty day list would look like a
          // valid edit and then fail the save with a message the row cannot
          // even show, so the link goes with its last day.
          if (weekdays.length === 0) return [];
          return [{ ...companion, weekdays }];
        }),
      );
    },
    [value, onChange],
  );

  const removeCompanion = useCallback(
    (companionId: string) => {
      onChange(
        value.filter(
          (companion) => companion.companion_student_id !== companionId,
        ),
      );
    },
    [value, onChange],
  );

  return (
    <div className="space-y-2">
      <p className="text-xs font-medium text-gray-700">Mit welchem Kind?</p>

      {value.length === 0 && !pickerOpen ? (
        <p className="text-xs text-gray-500">Noch kein Kind verknüpft.</p>
      ) : null}

      {value.map((companion) => (
        <div
          key={companion.companion_student_id}
          className="rounded-lg border border-gray-200 bg-white p-2.5"
        >
          <div className="mb-2 flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium text-gray-900">
              {companionDisplayName(companion)}
            </span>
            {!disabled && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`${companionDisplayName(companion)} entfernen`}
                onClick={() => removeCompanion(companion.companion_student_id)}
              >
                <X className="h-4 w-4" aria-hidden />
              </Button>
            )}
          </div>
          {/* Only the days the plan above allows are offered, so the two can
              never contradict each other. */}
          <div className="flex max-w-sm flex-wrap gap-1">
            {dayOptions.map((day) => {
              const active = companion.weekdays.includes(day.value);
              return (
                <label
                  key={day.value}
                  className={`flex h-8 w-16 items-center justify-center gap-1.5 rounded-lg border text-xs font-semibold transition-colors ${
                    disabled ? "cursor-default" : "cursor-pointer"
                  } ${
                    active
                      ? "border-gray-900 bg-white text-gray-900"
                      : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
                  }`}
                >
                  <Checkbox
                    checked={active}
                    disabled={disabled}
                    aria-label={`${companionDisplayName(companion)}: ${day.label}`}
                    onChange={() =>
                      toggleWeekday(companion.companion_student_id, day.value)
                    }
                  />
                  {day.short}
                </label>
              );
            })}
          </div>
        </div>
      ))}

      {pending ? (
        <div className="border-moto-orange bg-moto-orange/5 rounded-lg border p-2.5">
          <p className="text-xs text-gray-900">
            {pending.result.name} darf {formatDayList(pending.missingDays)} noch
            nicht mit anderen Kindern nach Hause gehen. Soll{" "}
            <span className="font-semibold">Anderes Kind</span> im Heimweg
            dieses Kindes ergänzt werden?
          </p>
          <div className="mt-2 flex justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              size="md"
              onClick={() => setPending(null)}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => {
                onExtensionConfirmed?.({
                  companion_student_id: pending.result.id,
                  weekdays: pending.missingDays,
                });
                commitCompanion(pending.result);
              }}
            >
              Ergänzen und verknüpfen
            </Button>
          </div>
        </div>
      ) : null}

      {!disabled && pickerOpen && !pending ? (
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
                    onClick={() => selectCompanion(result)}
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
              onClick={closePicker}
            >
              Abbrechen
            </Button>
          </div>
        </div>
      ) : null}

      {!disabled && !pickerOpen && !pending ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setPickerOpen(true)}
        >
          <Plus className="mr-1.5 h-4 w-4" aria-hidden />
          Kind hinzufügen
        </Button>
      ) : null}
    </div>
  );
}

/** Weekdays the candidate's own plan does not (yet) allow, in Mon..Fri order. */
function missingAccompaniedDays(
  allowed: AllowedDepartureModes | undefined,
  wanted: CompanionWeekday[],
): CompanionWeekday[] {
  return DEPARTURE_WEEKDAYS.filter(
    (day) =>
      wanted.includes(day.key as CompanionWeekday) &&
      !allowedModesIncludeAccompanied({ [day.key]: allowed?.[day.key] ?? [] }),
  ).map((day) => day.key as CompanionWeekday);
}

/** "montags", "montags und dienstags", "montags, dienstags und mittwochs". */
function formatDayList(days: CompanionWeekday[]): string {
  const labels = COMPANION_WEEKDAYS.filter((day) =>
    days.includes(day.value),
  ).map((day) => getGermanWeekdayAdverb(day.label));
  if (labels.length === 0) return "an diesen Tagen";
  if (labels.length === 1) return labels[0]!;
  return `${labels.slice(0, -1).join(", ")} und ${labels.at(-1)}`;
}
