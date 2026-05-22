"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { CalendarDays, Check, Clock, ShieldCheck } from "lucide-react";
import { useTenant } from "~/components/tenant/tenant-provider";
import {
  PublicEnrollmentBrand,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
  PublicInfoCard,
} from "~/components/enrollment/public-enrollment-shell";
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
 * Phase picker for the new public landing page for parents. Lists every
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
  const [enrollmentDisabled, setEnrollmentDisabled] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      setEnrollmentDisabled(false);
      try {
        const list = await fetchPublicPhases(tenantSlug);
        if (cancelled) return;
        setPhases(list);
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        const code = (err as { code?: string } | undefined)?.code;
        logger.error("phase_picker_load_failed", { error: message, code });
        if (code === "enrollment.disabled") {
          setEnrollmentDisabled(true);
        } else {
          setError(message);
        }
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
      <PublicEnrollmentPageShell>
        <div className="flex min-h-[70vh] items-center justify-center">
          <div className="moto-content-surface rounded-2xl border px-5 py-4 text-sm font-medium text-gray-600 shadow-sm">
            Anmeldung wird geladen...
          </div>
        </div>
      </PublicEnrollmentPageShell>
    );
  }

  return (
    <PublicEnrollmentPageShell>
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <PublicEnrollmentBrand tenant={tenant} />
        <PublicEnrollmentSteps current="phase" />
      </div>

      {error && (
        <div
          className="mb-6 rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      <section className="moto-content-surface overflow-hidden rounded-3xl border shadow-sm">
        <div className="grid lg:grid-cols-[minmax(0,1fr)_24rem]">
          <div className="p-5 sm:p-8 lg:p-10">
            <div className="max-w-3xl">
              <p className="text-sm font-semibold tracking-wide text-[#5080D8] uppercase">
                Anmeldung starten
              </p>
              <h1 className="mt-2 text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
                Wofür möchten Sie Ihr Kind anmelden?
              </h1>
              <p className="mt-4 max-w-2xl text-base leading-7 text-gray-600">
                Wählen Sie zuerst den passenden Zeitraum. Danach füllen Sie das
                Formular aus und erhalten einen Link, über den Sie den Status
                der Anmeldung ansehen können.
              </p>
            </div>

            <div className="mt-8">
              {!phases || phases.length === 0 ? (
                <div className="rounded-2xl border border-[#F78C10]/20 bg-[#F78C10]/10 p-5 text-sm leading-6 text-[#9A570A]">
                  {enrollmentDisabled
                    ? "Die Online-Anmeldung ist aktuell nicht freigeschaltet. Bitte wenden Sie sich an die OGS."
                    : "Aktuell ist keine Anmeldephase geöffnet. Bitte schauen Sie später wieder vorbei oder wenden Sie sich an die OGS."}
                </div>
              ) : (
                <ul className="space-y-3">
                  {phases.map((phase) => (
                    <li key={phase.id}>
                      <Link
                        href={`/enroll/${encodeURIComponent(phase.id)}`}
                        className="group moto-content-surface flex flex-col gap-4 rounded-2xl border p-5 text-left shadow-sm transition-all duration-150 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:flex-row sm:items-center sm:justify-between"
                      >
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-xs font-semibold text-gray-600">
                              {KIND_LABELS[phase.kind]}
                            </span>
                            {phase.enrollment_close_at && (
                              <span className="inline-flex items-center gap-1 rounded-full bg-[#83CD2D]/15 px-3 py-1 text-xs font-semibold text-[#5A8E1F]">
                                <Clock className="h-3.5 w-3.5" />
                                Offen bis{" "}
                                {formatDateTime(phase.enrollment_close_at)}
                              </span>
                            )}
                          </div>
                          <h2 className="mt-3 text-xl font-semibold text-gray-900">
                            {phase.name}
                          </h2>
                          <p className="mt-1 text-sm text-gray-600">
                            Betreuungszeitraum:{" "}
                            {formatDate(phase.service_start_date)} bis{" "}
                            {formatDate(phase.service_end_date)}
                          </p>
                        </div>
                        <span className="inline-flex h-9 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white transition-colors group-hover:bg-gray-800">
                          Anmeldung starten
                        </span>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-8 lg:border-t-0 lg:border-l">
            <div className="space-y-4 lg:sticky lg:top-8">
              <PublicInfoCard
                icon={<CalendarDays className="h-5 w-5" />}
                title="Zeitraum wählen"
              >
                Jede Anmeldung gehört zu einer Anmeldephase, zum Beispiel einem
                Schuljahr oder einer Ferienbetreuung.
              </PublicInfoCard>
              <PublicInfoCard
                icon={<Check className="h-5 w-5" />}
                title="Formular absenden"
              >
                Sie geben Ihre Kontaktdaten, die Angaben zum Kind und die
                gewünschten Betreuungsangebote an.
              </PublicInfoCard>
              <PublicInfoCard
                icon={<ShieldCheck className="h-5 w-5" />}
                title="Status verfolgen"
              >
                Nach dem Absenden erhalten Sie einen persönlichen Status-Link
                für Ihre Anmeldung.
              </PublicInfoCard>
            </div>
          </aside>
        </div>
      </section>
    </PublicEnrollmentPageShell>
  );
}

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatDateTime(value: string): string {
  return new Date(value).toLocaleString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
