"use client";

// Editor for a Jahrgangswechsel draft: one row per class with the target
// class or "Abgang" (child leaves the OGS, soft-deactivated as alumnus).

import { useEffect, useMemo, useState } from "react";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { createLogger } from "~/lib/logger";
import {
  createGradeTransition,
  fetchSuggestedMappings,
  fetchTransitionClasses,
  NOT_DRAFT_CODE,
  TransitionRequestError,
  updateGradeTransition,
  type GradeTransition,
  type MappingInput,
} from "~/lib/grade-transition-api";

const logger = createLogger({ component: "GradeTransitionEditor" });

export interface EditorRow {
  fromClass: string;
  mode: "promote" | "graduate";
  toClass: string;
  studentCount: number | null;
}

const MODE_OPTIONS = [
  { value: "promote", label: "Versetzen nach" },
  { value: "graduate", label: "Abgang (verlässt die OGS)" },
] as const;

/** Suggests the academic year label for the upcoming school year change. */
function suggestAcademicYear(now: Date): string {
  // From May onwards the transition targets the school year starting in
  // summer of the current year; before that it belongs to the previous one.
  const year = now.getMonth() >= 4 ? now.getFullYear() : now.getFullYear() - 1;
  return `${year}-${year + 1}`;
}

function rowsFromTransition(transition: GradeTransition): EditorRow[] {
  return transition.mappings.map((m) => ({
    fromClass: m.fromClass,
    mode: m.toClass === null ? "graduate" : "promote",
    toClass: m.toClass ?? "",
    studentCount: null,
  }));
}

interface TransitionEditorProps {
  readonly existingDraft: GradeTransition | null;
  readonly onClose: () => void;
  readonly onReadyForPreview: (transition: GradeTransition) => void;
}

export function TransitionEditor({
  existingDraft,
  onClose,
  onReadyForPreview,
}: TransitionEditorProps) {
  const [academicYear, setAcademicYear] = useState(
    existingDraft?.academicYear ?? suggestAcademicYear(new Date()),
  );
  const [notes, setNotes] = useState(existingDraft?.notes ?? "");
  const [rows, setRows] = useState<EditorRow[] | null>(
    existingDraft ? rowsFromTransition(existingDraft) : null,
  );
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [allClasses, setAllClasses] = useState<string[]>([]);
  const [classToAdd, setClassToAdd] = useState("");

  // All classes with children, so a row removed via "Nicht ändern" can be
  // added back (the suggestions are only the initial proposal).
  useEffect(() => {
    let cancelled = false;
    fetchTransitionClasses()
      .then((classes) => {
        if (!cancelled) setAllClasses(classes);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("classes_load_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (rows !== null) return;
    let cancelled = false;
    fetchSuggestedMappings()
      .then((suggestions) => {
        if (cancelled) return;
        setRows(
          suggestions.map((s) => ({
            fromClass: s.fromClass,
            // An ambiguous class (no grade pattern, e.g. "offen" or a free-form
            // name) must not be preselected as Abgang — default it to a
            // non-destructive "Versetzen" with an empty target so the admin has
            // to make an explicit choice before continuing.
            mode: !s.ambiguous && s.isGraduating ? "graduate" : "promote",
            toClass: s.ambiguous ? "" : (s.toClass ?? ""),
            studentCount: s.studentCount,
          })),
        );
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("suggestions_load_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setLoadError(
          "Vorschläge konnten nicht geladen werden. Bitte erneut versuchen.",
        );
      });
    return () => {
      cancelled = true;
    };
  }, [rows]);

  const updateRow = (index: number, patch: Partial<EditorRow>) => {
    setRows((current) =>
      (current ?? []).map((row, i) =>
        i === index ? { ...row, ...patch } : row,
      ),
    );
  };

  const removeRow = (index: number) => {
    setRows((current) => (current ?? []).filter((_, i) => i !== index));
  };

  const addRow = (fromClass: string) => {
    if (!fromClass) return;
    setRows((current) => {
      const next = current ?? [];
      if (next.some((r) => r.fromClass === fromClass)) return next;
      return [
        ...next,
        { fromClass, mode: "promote", toClass: "", studentCount: null },
      ];
    });
    setClassToAdd("");
  };

  // Classes with children that are not already a row — offered in the
  // "add class" selector so a removed class can be brought back.
  const addableClasses = useMemo(() => {
    const present = new Set((rows ?? []).map((r) => r.fromClass));
    return allClasses.filter((c) => !present.has(c));
  }, [allClasses, rows]);

  const validationError = useMemo(() => {
    if (!/^\d{4}-\d{4}$/.test(academicYear)) {
      return "Schuljahr im Format JJJJ-JJJJ angeben (z. B. 2026-2027).";
    }
    for (const row of rows ?? []) {
      if (row.mode === "promote") {
        const target = row.toClass.trim();
        if (!target) {
          return `Zielklasse für ${row.fromClass} fehlt.`;
        }
        if (target === row.fromClass) {
          return `Zielklasse für ${row.fromClass} darf nicht identisch sein.`;
        }
      }
    }
    if ((rows ?? []).length === 0) {
      return "Mindestens eine Klasse wird benötigt.";
    }
    return null;
  }, [academicYear, rows]);

  const handleContinue = async () => {
    if (validationError || !rows) return;
    setSaving(true);
    setSaveError(null);

    const mappings: MappingInput[] = rows.map((row) => ({
      fromClass: row.fromClass,
      toClass: row.mode === "graduate" ? null : row.toClass.trim(),
    }));

    try {
      const transition = existingDraft
        ? await updateGradeTransition(existingDraft.id, {
            academicYear,
            // Send the raw value (including "") so clearing the field actually
            // removes the stored note instead of leaving the old one in place.
            notes,
            mappings,
          })
        : await createGradeTransition({
            academicYear,
            notes: notes || undefined,
            mappings,
          });
      onReadyForPreview(transition);
    } catch (error) {
      logger.error("draft_save_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      // A not_draft conflict means another admin applied this draft since the
      // editor loaded: retrying can never succeed, so say what happened
      // instead of suggesting a retry (#405 review).
      setSaveError(
        error instanceof TransitionRequestError && error.code === NOT_DRAFT_CODE
          ? "Der Entwurf wurde inzwischen angewendet oder gelöscht und kann nicht mehr bearbeitet werden. Bitte das Fenster schließen und die Liste aktualisieren."
          : "Der Entwurf konnte nicht gespeichert werden. Bitte erneut versuchen.",
      );
      setSaving(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={
        existingDraft ? "Jahrgangswechsel bearbeiten" : "Neuer Jahrgangswechsel"
      }
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-3xl"
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            size="md"
            onClick={handleContinue}
            disabled={Boolean(validationError) || saving || !rows}
          >
            {saving ? "Speichern …" : "Weiter zur Vorschau"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label
              htmlFor="transition-academic-year"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Schuljahr (neu)
            </label>
            <Input
              id="transition-academic-year"
              value={academicYear}
              onChange={(e) => setAcademicYear(e.target.value)}
              placeholder="2026-2027"
            />
          </div>
          <div>
            <label
              htmlFor="transition-notes"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Notiz (optional)
            </label>
            <Input
              id="transition-notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="z. B. Schuljahreswechsel Sommer"
            />
          </div>
        </div>

        <p className="text-sm text-gray-600">
          Für jede Klasse festlegen, wohin die Kinder versetzt werden. Abgang
          bedeutet: Die Kinder verlassen die OGS und werden aus der App
          ausgeblendet. Das lässt sich über Zurücksetzen wieder rückgängig
          machen.
        </p>

        {loadError && <p className="text-moto-red text-sm">{loadError}</p>}
        {rows === null && !loadError && <Loading fullPage={false} />}

        {rows !== null && (
          <ul className="divide-y divide-gray-100 rounded-xl border border-gray-200">
            {rows.length === 0 && (
              <li className="p-4 text-sm text-gray-500">
                Keine Klassen mit Kindern gefunden.
              </li>
            )}
            {rows.map((row, index) => (
              <li
                key={row.fromClass}
                className="flex flex-wrap items-center gap-3 p-3"
              >
                <div className="w-20 shrink-0">
                  <span className="font-medium text-gray-900">
                    {row.fromClass}
                  </span>
                  {row.studentCount !== null && (
                    <span className="ml-1 text-xs text-gray-500">
                      ({row.studentCount})
                    </span>
                  )}
                </div>
                <div className="w-56 shrink-0">
                  <CustomSelect
                    ariaLabel={`Aktion für Klasse ${row.fromClass}`}
                    value={row.mode}
                    options={MODE_OPTIONS.map((o) => ({
                      value: o.value,
                      label: o.label,
                    }))}
                    onChange={(value) =>
                      updateRow(index, {
                        mode: value as EditorRow["mode"],
                      })
                    }
                  />
                </div>
                {row.mode === "promote" ? (
                  <div className="w-28">
                    <Input
                      aria-label={`Zielklasse für ${row.fromClass}`}
                      value={row.toClass}
                      onChange={(e) =>
                        updateRow(index, { toClass: e.target.value })
                      }
                      placeholder="z. B. 2a"
                    />
                  </div>
                ) : (
                  <span className="text-moto-red text-sm font-medium">
                    Abgang
                  </span>
                )}
                <div className="ml-auto">
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={() => removeRow(index)}
                    aria-label={`Klasse ${row.fromClass} nicht ändern`}
                  >
                    Nicht ändern
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        )}

        {rows !== null && addableClasses.length > 0 && (
          <div className="flex flex-wrap items-center gap-2">
            <div className="w-56">
              <CustomSelect
                ariaLabel="Klasse hinzufügen"
                value={classToAdd}
                placeholder="Klasse hinzufügen …"
                options={addableClasses.map((c) => ({ value: c, label: c }))}
                onChange={(value) => setClassToAdd(value)}
              />
            </div>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => addRow(classToAdd)}
              disabled={!classToAdd}
            >
              Hinzufügen
            </Button>
          </div>
        )}

        {validationError && rows !== null && rows.length > 0 && (
          <p className="text-moto-orange text-sm">{validationError}</p>
        )}
        {saveError && <p className="text-moto-red text-sm">{saveError}</p>}
      </div>
    </Modal>
  );
}
