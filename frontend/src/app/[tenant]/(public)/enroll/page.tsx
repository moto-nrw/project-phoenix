"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTenant } from "~/components/tenant/tenant-provider";
import {
  fetchPublicPhases,
  type PublicPhase,
} from "~/lib/enrollment-submission-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollPhasePicker" });

const KIND_LABELS: Record<PublicPhase["kind"], string> = {
  school_year: "Schuljahr",
  holiday: "Ferienbetreuung",
  custom: "Sonstiges",
};

/**
 * Phase picker — the new public landing page for parents. Lists every
 * currently-open phase as a card. Clicking a phase navigates to
 * /{tenant}/enroll/{phaseId} which renders the per-phase form. Was
 * previously a school-year dropdown above the form; PR B's phase
 * model splits the picker out so each phase carries its own form.
 */
export default function EnrollPhasePickerPage() {
  const { tenantSlug, tenant } = useTenant();
  const [phases, setPhases] = useState<PublicPhase[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const list = await fetchPublicPhases(tenantSlug);
        if (cancelled) return;
        setPhases(list);
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("phase_picker_load_failed", { error: message });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [tenantSlug]);

  if (loading) {
    return (
      <main className="mx-auto max-w-3xl p-6 text-sm text-gray-500">
        Wird geladen...
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-3xl space-y-6 p-6">
      <header className="space-y-2">
        <h1 className="text-2xl font-semibold text-gray-900">
          Anmeldung an der {tenant?.name ?? "Schule"}
        </h1>
        <p className="text-sm text-gray-600">
          Wähle aus, wofür du dein Kind anmelden möchtest. Du kannst pro Phase
          eine eigene Anmeldung absenden.
        </p>
      </header>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800">
          {error}
        </div>
      )}

      {!phases || phases.length === 0 ? (
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
          Aktuell ist keine Anmeldephase geöffnet. Bitte komm später wieder oder
          wende dich an die Schulleitung.
        </div>
      ) : (
        <ul className="grid gap-3">
          {phases.map((p) => (
            <li
              key={p.id}
              className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition-shadow hover:shadow-md"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="text-xs font-medium text-[#5a8e1f] uppercase">
                    {KIND_LABELS[p.kind]}
                  </p>
                  <h2 className="mt-1 text-lg font-semibold text-gray-900">
                    {p.name}
                  </h2>
                  <p className="mt-2 text-sm text-gray-600">
                    Betreuungszeitraum: {p.service_start_date} –{" "}
                    {p.service_end_date}
                  </p>
                  {p.enrollment_close_at && (
                    <p className="text-xs text-gray-500">
                      Anmeldungen möglich bis{" "}
                      {new Date(p.enrollment_close_at).toLocaleString("de-DE", {
                        day: "2-digit",
                        month: "long",
                        year: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </p>
                  )}
                </div>
                <Link
                  href={`/enroll/${encodeURIComponent(p.id)}`}
                  className="rounded-md bg-[#83CD2D] px-4 py-2 text-sm font-medium text-white hover:opacity-90"
                >
                  Anmelden →
                </Link>
              </div>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
