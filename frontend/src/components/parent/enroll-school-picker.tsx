"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { type EnrollablePhase, listEnrollableSchools } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollSchoolPicker" });

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
    });
  } catch {
    return iso;
  }
}

interface SchoolGroup {
  readonly schoolSlug: string;
  readonly schoolName: string;
  readonly alreadyLinked: boolean;
  readonly phases: EnrollablePhase[];
}

function groupPhasesBySchool(
  phases: readonly EnrollablePhase[],
): SchoolGroup[] {
  const groups = new Map<string, SchoolGroup>();
  for (const p of phases) {
    const existing = groups.get(p.school_slug);
    if (existing) {
      existing.phases.push(p);
      continue;
    }
    groups.set(p.school_slug, {
      schoolSlug: p.school_slug,
      schoolName: p.school_name,
      alreadyLinked: p.already_linked,
      phases: [p],
    });
  }
  return [...groups.values()];
}

interface PhaseCardProps {
  readonly phase: EnrollablePhase;
}

function PhaseCard({ phase }: PhaseCardProps) {
  return (
    <Link
      href={`/parents/enroll/${phase.school_slug}/${phase.phase_id}`}
      className="flex items-center justify-between rounded-xl border border-gray-100 px-4 py-3 transition-colors hover:bg-gray-50"
    >
      <div>
        <p className="text-sm font-medium text-gray-900">{phase.phase_name}</p>
        <p className="text-xs text-gray-600">
          Betreuung {formatDate(phase.service_start_date)} –{" "}
          {formatDate(phase.service_end_date)}
        </p>
      </div>
      <span className="text-sm text-[#83CD2D]">Anmelden →</span>
    </Link>
  );
}

interface SchoolSectionProps {
  readonly group: SchoolGroup;
}

function SchoolSection({ group }: SchoolSectionProps) {
  return (
    <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
      <header className="flex items-center justify-between">
        <h2 className="text-base font-semibold text-gray-900">
          {group.schoolName}
        </h2>
        {group.alreadyLinked ? (
          <span
            className="rounded-full px-3 py-1 text-xs font-medium"
            style={{ backgroundColor: "#83CD2D", color: "#FFFFFF" }}
          >
            Bereits ein Kind angemeldet
          </span>
        ) : null}
      </header>
      <ul className="mt-3 space-y-3">
        {group.phases.map((p) => (
          <li key={p.phase_id}>
            <PhaseCard phase={p} />
          </li>
        ))}
      </ul>
    </section>
  );
}

export function EnrollSchoolPicker() {
  const [phases, setPhases] = useState<EnrollablePhase[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listEnrollableSchools();
      setPhases(list);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("enrollable_schools_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const groups = useMemo(() => groupPhasesBySchool(phases), [phases]);

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }
  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        {error}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <header>
        <Link
          href="/parents"
          className="text-sm text-gray-500 hover:text-gray-700"
        >
          ← Zurück zur Übersicht
        </Link>
        <h1 className="mt-2 text-2xl font-semibold text-gray-900">
          Neue Anmeldung
        </h1>
        <p className="mt-1 text-sm text-gray-600">
          {groups.length === 0
            ? "Aktuell sind keine Anmeldephasen geöffnet."
            : "Wählen Sie die Schule und Phase aus, für die Sie Ihr Kind anmelden möchten."}
        </p>
      </header>

      {groups.length === 0 ? (
        <div className="rounded-2xl border border-gray-200 bg-white p-8 text-center text-sm text-gray-600 shadow-sm">
          Sobald eine Schule eine Anmeldephase öffnet, erscheint sie hier.
        </div>
      ) : (
        <div className="space-y-4">
          {groups.map((g) => (
            <SchoolSection key={g.schoolSlug} group={g} />
          ))}
        </div>
      )}
    </div>
  );
}
