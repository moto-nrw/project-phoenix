"use client";

// Klassen-Zuweisung (#1772): welche Schulklassen eine Lehrkraft betreut.
// Die Zuweisung scopt die Lehrkraft-Klassenansicht (/klassen) — deshalb ist
// das Ersetzen des Sets im Backend users:manage (Admin-Write mit Audit).
// Klassen bleiben Freitext (users.students.school_class); die Vorschläge
// kommen aus den vorhandenen Klassen der Kinder.

import { Plus, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { authFetch } from "~/lib/api-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffKlassenTab" });

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

export function KlassenTab({
  staffId,
  canEdit,
}: Readonly<{ staffId: string; canEdit: boolean }>) {
  const [assigned, setAssigned] = useState<string[] | null>(null);
  const [known, setKnown] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchAssignedClasses(staffId)
      .then((classes) => {
        if (!cancelled) setAssigned(classes);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError("Die Klassen-Zuweisung konnte nicht geladen werden.");
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
  }, [staffId]);

  const persist = useCallback(
    async (next: string[]) => {
      const previous = assigned ?? [];
      setAssigned(next);
      setSaving(true);
      setError(null);
      try {
        const stored = await saveAssignedClasses(staffId, next);
        setAssigned(stored);
      } catch (err) {
        setAssigned(previous);
        setError("Die Klassen-Zuweisung konnte nicht gespeichert werden.");
        logger.error("staff_school_classes_save_failed", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
      } finally {
        setSaving(false);
      }
    },
    [assigned, staffId],
  );

  const addClass = useCallback(() => {
    const value = draft.trim();
    if (!value || !assigned) return;
    const exists = assigned.some(
      (klass) => klass.trim().toLowerCase() === value.toLowerCase(),
    );
    setDraft("");
    if (exists) return;
    void persist([...assigned, value]);
  }, [draft, assigned, persist]);

  const removeClass = useCallback(
    (klass: string) => {
      if (!assigned) return;
      void persist(assigned.filter((entry) => entry !== klass));
    },
    [assigned, persist],
  );

  const suggestions = useMemo(() => {
    if (!assigned) return [];
    const taken = new Set(assigned.map((klass) => klass.trim().toLowerCase()));
    return known.filter((klass) => !taken.has(klass.trim().toLowerCase()));
  }, [known, assigned]);

  return (
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
      <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
        Klassen
      </p>
      <h2 className="mt-1 text-base font-semibold text-gray-900">
        Zugewiesene Schulklassen
      </h2>
      <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
        Eine Lehrkraft sieht in ihrer Klassenansicht genau die Kinder dieser
        Klassen – wer nach dem Unterricht in der Betreuung bleibt und wer nach
        Hause geht. Kontaktdaten der Sorgeberechtigten sind dort nicht sichtbar.
      </p>

      {error && (
        <div className="mt-4">
          <Alert type="error" message={error} />
        </div>
      )}

      {assigned === null && !error ? (
        <div className="mt-4 space-y-2">
          <Skeleton className="h-9 w-64" />
          <Skeleton className="h-9 w-40" />
        </div>
      ) : null}

      {assigned !== null && (
        <>
          <div className="mt-4 flex flex-wrap gap-2">
            {assigned.length === 0 && (
              <p className="text-sm text-gray-500">
                Noch keine Klasse zugewiesen.
              </p>
            )}
            {assigned.map((klass) => (
              <span
                key={klass}
                className="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-900 shadow-sm"
              >
                Klasse {klass}
                {canEdit && (
                  <button
                    type="button"
                    aria-label={`Klasse ${klass} entfernen`}
                    onClick={() => removeClass(klass)}
                    disabled={saving}
                    className="rounded p-0.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 disabled:opacity-50"
                  >
                    <X className="h-3.5 w-3.5" aria-hidden="true" />
                  </button>
                )}
              </span>
            ))}
          </div>

          {canEdit && (
            <div className="mt-4 flex items-center gap-2">
              <input
                type="text"
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
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
                className="h-9 w-40 rounded-lg border border-gray-200 bg-white px-3 text-sm text-gray-900 shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:bg-gray-50"
              />
              <datalist id="staff-known-classes">
                {suggestions.map((klass) => (
                  <option key={klass} value={klass} />
                ))}
              </datalist>
              <button
                type="button"
                onClick={addClass}
                disabled={saving || draft.trim() === ""}
                className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Hinzufügen
              </button>
            </div>
          )}
        </>
      )}
    </section>
  );
}
