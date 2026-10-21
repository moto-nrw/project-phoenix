"use client";

import { useEffect, useState } from "react";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EmptyState } from "~/components/ui/empty-state";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { Radio } from "~/components/ui/radio";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { fetchStudents } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";

const logger = createLogger({ component: "StudentChoiceList" });

const STUDENT_PAGE_SIZE = 100;

export interface StudentChoice {
  readonly id: string;
  readonly name: string;
  readonly detail: string;
}

// Alle Kinder der Schule in einem Rutsch und alphabetisch: die Liste steht
// beim Öffnen und filtert ab dem ersten Zeichen ohne weitere Anfrage
// (#4355, #4362).
async function fetchStudentChoices(): Promise<StudentChoice[]> {
  const choices: StudentChoice[] = [];
  for (let page = 1; ; page += 1) {
    const result = await fetchStudents({ page, page_size: STUDENT_PAGE_SIZE });
    choices.push(...result.students.map(toChoice));
    if (!result.pagination || page >= result.pagination.total_pages) {
      return choices.sort((a, b) => a.name.localeCompare(b.name, "de"));
    }
  }
}

function toChoice(student: Student): StudentChoice {
  return {
    id: student.id.toString(),
    name:
      student.name ||
      [student.first_name, student.second_name].filter(Boolean).join(" "),
    detail: [student.school_class, student.group_name]
      .filter(Boolean)
      .join(" · "),
  };
}

// Each result names the scope it was loaded for: after a scope switch (a
// block in the supervision view) the old list must not stay selectable for
// even one render.
export type StudentChoicesLoad =
  | { readonly kind: "loading" }
  | { readonly kind: "failed"; readonly scope: string }
  | {
      readonly kind: "loaded";
      readonly scope: string;
      readonly choices: readonly StudentChoice[];
    };

/**
 * Loads every child of the school while `enabled`. A changed `scope` loads
 * again and hides the previous result meanwhile.
 */
export function useStudentChoices(
  enabled: boolean,
  scope = "",
): { readonly load: StudentChoicesLoad; readonly error: FormErrorInput } {
  const [result, setResult] = useState<StudentChoicesLoad>({
    kind: "loading",
  });
  const load: StudentChoicesLoad =
    result.kind !== "loading" && result.scope !== scope
      ? { kind: "loading" }
      : result;
  const [reloadCount, setReloadCount] = useState(0);
  const { error, show: showError, clear: clearError } = useApiLoadError();

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setResult({ kind: "loading" });
    clearError();
    fetchStudentChoices()
      .then((choices) => {
        if (!cancelled) setResult({ kind: "loaded", scope, choices });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.warn("student_choices_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setResult({ kind: "failed", scope });
        void showError(err, {
          object: "die Kinder",
          retry: () => setReloadCount((count) => count + 1),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [enabled, scope, reloadCount, clearError, showError]);

  return { load, error };
}

export interface StudentChoiceMatch {
  readonly term: string;
  readonly visible: readonly StudentChoice[];
  readonly selectable: readonly StudentChoice[];
  readonly selected: StudentChoice | null;
  /** The explicit choice, or the only selectable child left. */
  readonly target: StudentChoice | null;
}

export function matchStudentChoices(
  load: StudentChoicesLoad,
  search: string,
  selectedId: string | null,
  unavailableIds: ReadonlySet<string> = new Set(),
): StudentChoiceMatch {
  const term = search.trim().toLocaleLowerCase("de");
  const choices = load.kind === "loaded" ? load.choices : [];
  const visible = term
    ? choices.filter((child) =>
        child.name.toLocaleLowerCase("de").includes(term),
      )
    : choices;
  const selectable = visible.filter((child) => !unavailableIds.has(child.id));
  // Derived against the visible list so a selection the filter hides, or a
  // child that became unavailable meanwhile, can never be taken by mistake.
  const selected = selectable.find((child) => child.id === selectedId) ?? null;
  const target =
    selected ?? (selectable.length === 1 ? (selectable[0] ?? null) : null);
  return { term, visible, selectable, selected, target };
}

export interface StudentChoiceListProps {
  readonly load: StudentChoicesLoad;
  readonly loadError: FormErrorInput;
  readonly search: string;
  readonly onSearchChange: (value: string) => void;
  readonly searchLabel: string;
  readonly radioName: string;
  readonly selectedId: string | null;
  readonly onSelectedIdChange: (id: string | null) => void;
  /** Children shown in the list that cannot be picked here. */
  readonly unavailableIds?: ReadonlySet<string>;
  /** Short reason next to an unavailable child, e.g. „schon dabei“. */
  readonly unavailableLabel?: string;
  readonly disabled?: boolean;
  readonly autoFocus?: boolean;
}

/**
 * Search field plus a radio list of every child, alphabetical. One tab stop,
 * arrow keys move the choice, unavailable children are skipped. The count and
 * an empty result are announced while typing. Used by „Kind ungeplant
 * hinzufügen“ and „Neue Nachricht“.
 */
export function StudentChoiceList({
  load,
  loadError,
  search,
  onSearchChange,
  searchLabel,
  radioName,
  selectedId,
  onSelectedIdChange,
  unavailableIds,
  unavailableLabel,
  disabled = false,
  autoFocus,
}: StudentChoiceListProps) {
  const { term, visible, selectable, selected, target } = matchStudentChoices(
    load,
    search,
    selectedId,
    unavailableIds,
  );

  return (
    <>
      <Input
        type="search"
        name={`${radioName}-search`}
        aria-label={searchLabel}
        controlSize="compact"
        value={search}
        onChange={(event) => {
          onSelectedIdChange(null);
          onSearchChange(event.target.value);
        }}
        placeholder="Name suchen..."
        autoFocus={autoFocus}
      />
      {load.kind === "loading" ? (
        <Loading message="Kinder werden geladen..." fullPage={false} />
      ) : null}
      {load.kind === "failed" ? <LoadErrorAlert error={loadError} /> : null}
      {/* Die Zahl und der leere Treffer werden beim Tippen angesagt. */}
      <div role="status">
        {load.kind === "loaded" && visible.length === 0 ? (
          <EmptyState
            variant="compact"
            title={
              term
                ? "Kein Kind mit diesem Namen gefunden."
                : "Es sind noch keine Kinder angelegt."
            }
          />
        ) : null}
        {visible.length > 0 ? (
          <p className="text-sm text-gray-600">
            {visible.length === 1 ? "1 Kind" : `${visible.length} Kinder`}
          </p>
        ) : null}
      </div>
      {visible.length > 0 ? (
        <fieldset>
          <legend className="sr-only">Kind auswählen</legend>
          <ul className="grid max-h-80 gap-2 overflow-y-auto">
            {visible.map((child) => {
              const unavailable = unavailableIds?.has(child.id) ?? false;
              const checked = target?.id === child.id;
              return (
                <li key={child.id}>
                  <ChoiceTile
                    selected={checked}
                    disabled={unavailable || disabled}
                    tone="green"
                  >
                    <Radio
                      name={radioName}
                      value={child.id}
                      checked={checked}
                      disabled={unavailable || disabled}
                      onChange={() => onSelectedIdChange(child.id)}
                    />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-medium text-gray-900">
                        {child.name}
                      </span>
                      <span className="block truncate text-gray-500">
                        {child.detail || "–"}
                      </span>
                    </span>
                    {unavailable && unavailableLabel ? (
                      <span className="shrink-0 text-gray-600">
                        {unavailableLabel}
                      </span>
                    ) : null}
                  </ChoiceTile>
                </li>
              );
            })}
          </ul>
        </fieldset>
      ) : null}
      {selectable.length > 1 && !selected ? (
        <p className="text-sm text-gray-500">
          Bitte ein Kind aus der Liste antippen.
        </p>
      ) : null}
    </>
  );
}
