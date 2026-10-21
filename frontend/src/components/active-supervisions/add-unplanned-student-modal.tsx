"use client";

import { useEffect, useId, useState } from "react";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EmptyState } from "~/components/ui/empty-state";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { Radio } from "~/components/ui/radio";
import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { fetchStudents } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";

const logger = createLogger({ component: "AddUnplannedStudentModal" });

const STUDENT_PAGE_SIZE = 100;

interface UnplannedCandidate {
  readonly id: string;
  readonly name: string;
  readonly detail: string;
}

// Alle Kinder, die die Suche bisher finden konnte, in einem Rutsch und
// alphabetisch: die Liste steht beim Öffnen und filtert ab dem ersten
// Zeichen ohne weitere Anfrage (#4355).
async function fetchCandidates(): Promise<UnplannedCandidate[]> {
  const candidates: UnplannedCandidate[] = [];
  for (let page = 1; ; page += 1) {
    const result = await fetchStudents({ page, page_size: STUDENT_PAGE_SIZE });
    candidates.push(...result.students.map(toCandidate));
    if (!result.pagination || page >= result.pagination.total_pages) {
      return candidates.sort((a, b) => a.name.localeCompare(b.name, "de"));
    }
  }
}

function toCandidate(student: Student): UnplannedCandidate {
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

// Each result names the block it was loaded for: after a block switch the
// old list must not stay selectable for even one render.
type LoadState =
  | { readonly kind: "loading" }
  | { readonly kind: "failed"; readonly instanceId: string }
  | {
      readonly kind: "loaded";
      readonly instanceId: string;
      readonly candidates: readonly UnplannedCandidate[];
    };

export interface AddUnplannedStudentModalProps {
  readonly isOpen: boolean;
  readonly instanceId: string;
  /** Kinder, die gerade schon in diesem Block sind; sie stehen nicht zur Wahl. */
  readonly inBlockStudentIds: ReadonlySet<string>;
  readonly isAddingStudent: boolean;
  readonly search: string;
  readonly error: FormErrorInput;
  readonly onAdd: (studentId: string) => Promise<boolean>;
  readonly onClose: () => void;
  readonly onSearchChange: (value: string) => void;
}

// Das Nachtragen eines Kindes ist eine Kopf-Aktion mit Dialog, kein
// Formular im Listenkörper (#3112, Bauart 1 Regel 3): die Liste bleibt eine
// Liste, und „Hinzufügen“ steht unten im Dialog, wo eine Aktion hingehört.
export function AddUnplannedStudentModal({
  isOpen,
  instanceId,
  inBlockStudentIds,
  isAddingStudent,
  search,
  error,
  onAdd,
  onClose,
  onSearchChange,
}: AddUnplannedStudentModalProps) {
  const formId = useId();
  const [loadResult, setLoad] = useState<LoadState>({ kind: "loading" });
  const load: LoadState =
    loadResult.kind !== "loading" && loadResult.instanceId !== instanceId
      ? { kind: "loading" }
      : loadResult;
  const [reloadCount, setReloadCount] = useState(0);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // Ein Termin-Wechsel bei offenem Dialog verwirft die Auswahl: eine alte
  // Auswahl darf nie in den neuen Termin geschrieben werden.
  const [selectionInstanceId, setSelectionInstanceId] = useState(instanceId);
  if (selectionInstanceId !== instanceId) {
    setSelectionInstanceId(instanceId);
    setSelectedId(null);
  }

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setLoad({ kind: "loading" });
    clearLoadError();
    fetchCandidates()
      .then((candidates) => {
        if (!cancelled) setLoad({ kind: "loaded", instanceId, candidates });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.warn("failed to load students for unplanned add", {
          error: err instanceof Error ? err.message : String(err),
        });
        setLoad({ kind: "failed", instanceId });
        void showLoadError(err, {
          object: "die Kinder",
          retry: () => setReloadCount((count) => count + 1),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, instanceId, reloadCount, clearLoadError, showLoadError]);

  const term = search.trim().toLocaleLowerCase("de");
  const candidates = load.kind === "loaded" ? load.candidates : [];
  const visible = term
    ? candidates.filter((child) =>
        child.name.toLocaleLowerCase("de").includes(term),
      )
    : candidates;
  const selectable = visible.filter(
    (child) => !inBlockStudentIds.has(child.id),
  );
  // Derived against the visible list so a selection the filter hides, or a
  // child that joined the block meanwhile, can never be added by mistake.
  const selected = selectable.find((child) => child.id === selectedId) ?? null;
  const target =
    selected ?? (selectable.length === 1 ? (selectable[0] ?? null) : null);

  const handleClose = () => {
    setSelectedId(null);
    onClose();
  };
  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (target && !isAddingStudent && (await onAdd(target.id))) {
      handleClose();
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleClose}
      title="Kind ungeplant hinzufügen"
      size="md"
      error={error}
      closeDisabled={isAddingStudent}
      footer={
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={handleClose}
            disabled={isAddingStudent}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form={formId}
            disabled={isAddingStudent || !target}
            variant="success"
            size="md"
          >
            {isAddingStudent ? "Wird hinzugefügt…" : "Hinzufügen"}
          </Button>
        </div>
      }
    >
      <form id={formId} onSubmit={handleSubmit} className="space-y-3">
        <p className="text-sm text-gray-600">
          Das Kind wird sofort als anwesend in dieser Aktivität eingetragen.
        </p>
        <Input
          type="search"
          name="unplanned-student-search"
          aria-label="Kind ungeplant suchen"
          controlSize="compact"
          value={search}
          onChange={(event) => {
            setSelectedId(null);
            onSearchChange(event.target.value);
          }}
          placeholder="Name suchen..."
        />
        {load.kind === "loading" ? (
          <Loading message="Kinder werden geladen..." fullPage={false} />
        ) : null}
        {load.kind === "failed" ? <LoadErrorAlert error={loadError} /> : null}
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
          <fieldset className="space-y-2">
            <legend className="text-sm text-gray-600">
              {visible.length === 1 ? "1 Kind" : `${visible.length} Kinder`}
            </legend>
            <ul className="grid max-h-80 gap-2 overflow-y-auto">
              {visible.map((child) => {
                const inBlock = inBlockStudentIds.has(child.id);
                const checked = target?.id === child.id;
                return (
                  <li key={child.id}>
                    <ChoiceTile
                      selected={checked}
                      disabled={inBlock || isAddingStudent}
                      tone="green"
                    >
                      <Radio
                        name="unplanned-student"
                        value={child.id}
                        checked={checked}
                        disabled={inBlock || isAddingStudent}
                        onChange={() => setSelectedId(child.id)}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium text-gray-900">
                          {child.name}
                        </span>
                        <span className="block truncate text-gray-500">
                          {child.detail || "–"}
                        </span>
                      </span>
                      {inBlock ? (
                        <span className="shrink-0 text-gray-600">
                          schon dabei
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
      </form>
    </FormModal>
  );
}
