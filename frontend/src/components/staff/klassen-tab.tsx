"use client";

// Klassen-Zuweisung (#1772): welche Schulklassen eine Lehrkraft betreut.
// Die Zuweisung scopt die Klassenansicht in moto schule, deshalb ist
// das Ersetzen des Sets im Backend users:manage (Admin-Write mit Audit).
// Klassen bleiben Freitext (users.students.school_class); die Vorschläge
// kommen aus den vorhandenen Klassen der Kinder.
//
// Bearbeitet wird am Objekt (Bauart 2, Regel 4): „Bearbeiten“ öffnet einen
// Entwurf, Hinzufügen und Entfernen ändern nur den Entwurf, und erst
// „Speichern“ schreibt das ganze Set einmal per PUT (#3112). Vorher schrieb
// jeder Chip-Klick sofort, ohne Abbrechen und ohne Hinweis.

import { Plus, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { authFetch } from "~/lib/api-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffKlassenTab" });

// Klassennamen sind Freitext — manche Schulen speichern "1a", andere schon
// "Klasse 1a". Kein doppeltes Präfix anzeigen.
function classLabel(klass: string): string {
  return /^klasse\b/i.test(klass.trim()) ? klass.trim() : `Klasse ${klass}`;
}

interface ProxyResponse<T> {
  success: boolean;
  data: T;
}

interface SchoolClassesPayload {
  staff_id: number;
  school_classes: string[];
}

async function fetchAssignedClasses(staffId: string): Promise<string[]> {
  const response = await authFetch<ProxyResponse<SchoolClassesPayload>>(
    `/api/staff/${staffId}/school-classes`,
  );
  return response.data.school_classes ?? [];
}

async function saveAssignedClasses(
  staffId: string,
  classes: string[],
): Promise<string[]> {
  const response = await authFetch<ProxyResponse<SchoolClassesPayload>>(
    `/api/staff/${staffId}/school-classes`,
    { method: "PUT", body: { school_classes: classes } },
  );
  return response.data.school_classes ?? [];
}

async function fetchKnownClasses(): Promise<string[]> {
  const response = await authFetch<{ success: boolean; data: string[] }>(
    "/api/students/school-classes",
  );
  return response.data ?? [];
}

function sameClassSet(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((klass, index) => klass === b[index]);
}

export function KlassenTab({
  staffId,
  canEdit,
}: Readonly<{ staffId: string; canEdit: boolean }>) {
  const [assigned, setAssigned] = useState<string[] | null>(null);
  const [known, setKnown] = useState<string[]>([]);
  // Entwurf des Bearbeiten-Zustands; null heißt: nicht im Bearbeiten.
  const [draft, setDraft] = useState<string[] | null>(null);
  const [newClass, setNewClass] = useState("");
  const [saving, setSaving] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  // Zählt die Ladeversuche: ein fehlgeschlagener Initial-Load (transienter
  // 500) darf die Zuweisungs-UI nicht dauerhaft sperren — "Erneut versuchen"
  // stößt den Effect neu an, statt einen Full Reload zu erzwingen.
  const [loadAttempt, setLoadAttempt] = useState(0);

  useEffect(() => {
    let cancelled = false;
    fetchAssignedClasses(staffId)
      .then((classes) => {
        if (!cancelled) setAssigned(classes);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setLoadError("Die Klassen-Zuweisung konnte nicht geladen werden.");
        logger.error("staff_school_classes_fetch_failed", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
      });
    fetchKnownClasses()
      .then((classes) => {
        if (!cancelled) setKnown(classes);
      })
      .catch(() => {
        // Vorschläge sind optional — Freitext funktioniert weiterhin.
      });
    return () => {
      cancelled = true;
    };
  }, [staffId, loadAttempt]);

  const retryLoad = useCallback(() => {
    setLoadError(null);
    setLoadAttempt((attempt) => attempt + 1);
  }, []);

  const startEditing = useCallback(() => {
    if (!assigned) return;
    setDraft([...assigned]);
    setNewClass("");
    setSaveError(null);
  }, [assigned]);

  const cancelEditing = useCallback(() => {
    setDraft(null);
    setNewClass("");
    setSaveError(null);
  }, []);

  const addClass = useCallback(() => {
    const value = newClass.trim();
    if (!value || !draft) return;
    setNewClass("");
    const exists = draft.some(
      (klass) => klass.trim().toLowerCase() === value.toLowerCase(),
    );
    if (exists) return;
    setDraft([...draft, value]);
  }, [newClass, draft]);

  const removeClass = useCallback((klass: string) => {
    setDraft((current) =>
      current ? current.filter((entry) => entry !== klass) : current,
    );
  }, []);

  const handleSave = useCallback(async () => {
    if (!draft) return;
    setSaving(true);
    setSaveError(null);
    try {
      const stored = await saveAssignedClasses(staffId, draft);
      setAssigned(stored);
      setDraft(null);
      setNewClass("");
    } catch (err) {
      setSaveError("Die Klassen-Zuweisung konnte nicht gespeichert werden.");
      logger.error("staff_school_classes_save_failed", {
        staff_id: staffId,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setSaving(false);
    }
  }, [draft, staffId]);

  const suggestions = useMemo(() => {
    const base = draft ?? assigned ?? [];
    const taken = new Set(base.map((klass) => klass.trim().toLowerCase()));
    return known.filter((klass) => !taken.has(klass.trim().toLowerCase()));
  }, [known, draft, assigned]);

  const editing = draft !== null;
  const shown = draft ?? assigned;
  const dirty =
    draft !== null && assigned !== null && !sameClassSet(draft, assigned);

  return (
    <SectionCard
      title="Zugewiesene Schulklassen"
      description="Eine Lehrkraft sieht in ihrer Klassenansicht genau die Kinder dieser Klassen – wer nach dem Unterricht in der Betreuung bleibt und wer nach Hause geht. Kontaktdaten der Sorgeberechtigten sind dort nicht sichtbar."
      action={
        canEdit && assigned !== null && !editing ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            className="bg-white"
            onClick={startEditing}
          >
            Bearbeiten
          </Button>
        ) : undefined
      }
    >
      {loadError && (
        <div className="space-y-3">
          <Alert type="error" message={loadError} />
          {assigned === null && (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={retryLoad}
            >
              Erneut versuchen
            </Button>
          )}
        </div>
      )}

      {assigned === null && !loadError ? (
        <div className="space-y-2">
          <Skeleton className="h-9 w-64" />
          <Skeleton className="h-9 w-40" />
        </div>
      ) : null}

      {shown !== null && (
        <div className="space-y-4">
          {editing && saveError && <Alert type="error" message={saveError} />}

          <div className="flex flex-wrap gap-2">
            {shown.length === 0 && (
              <p className="text-sm text-gray-500">
                Noch keine Klasse zugewiesen.
              </p>
            )}
            {shown.map((klass) => (
              <span
                key={klass}
                // min-h-8 hält die Chip-Höhe konstant, egal ob der
                // Entfernen-Button (h-8) daneben steht.
                className={`inline-flex min-h-8 items-center gap-1 rounded-lg border border-gray-200 bg-white pl-3 text-sm font-medium text-gray-900 shadow-sm ${editing ? "pr-1" : "pr-3"}`}
              >
                {classLabel(klass)}
                {editing && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label={`Klasse ${klass} entfernen`}
                    title={`Klasse ${klass} entfernen`}
                    onClick={() => removeClass(klass)}
                    disabled={saving}
                  >
                    <X className="h-3.5 w-3.5" aria-hidden="true" />
                  </Button>
                )}
              </span>
            ))}
          </div>

          {editing && (
            <>
              <div className="flex items-center gap-2">
                <div className="w-40">
                  <Input
                    type="text"
                    value={newClass}
                    onChange={(event) => setNewClass(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        event.preventDefault();
                        addClass();
                      }
                    }}
                    list="staff-known-classes"
                    placeholder="z. B. 1a"
                    aria-label="Klassenname"
                    disabled={saving}
                    controlSize="compact"
                  />
                </div>
                <datalist id="staff-known-classes">
                  {suggestions.map((klass) => (
                    <option key={klass} value={klass} />
                  ))}
                </datalist>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={addClass}
                  disabled={saving || newClass.trim() === ""}
                  className="gap-1.5 bg-white"
                >
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  Hinzufügen
                </Button>
              </div>
              <EditActions
                onCancel={cancelEditing}
                onSave={() => void handleSave()}
                saving={saving}
                disabled={!dirty}
              />
            </>
          )}
        </div>
      )}
    </SectionCard>
  );
}
