"use client";

import { use, useEffect, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- public page; tenant-router not needed
import { useRouter } from "next/navigation";
import { CalendarDays, Check, Mail, ShieldCheck } from "lucide-react";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import {
  PublicEnrollmentBackLink,
  PublicEnrollmentBrand,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
  PublicInfoCard,
} from "~/components/enrollment/public-enrollment-shell";
import { useTenant } from "~/components/tenant/tenant-provider";
import {
  fetchPublicPhases,
  type PublicPhase,
} from "~/lib/enrollment-submission-api";

interface PageProps {
  readonly params: Promise<{ tenant: string; phaseId: string }>;
}

/**
 * Per-phase enrollment form page. The parent picks a phase on the
 * landing page (`/enroll`); this page renders the form scoped to that
 * phase. The form fetches its own schema + offerings using the phaseId,
 * and posts to /api/enrollment/{tenantSlug}/submit with phase_id set.
 *
 * Grade-level cap is hardcoded to 4 (OGS default), same as the
 * pre-phase page. PR D autofill or per-phase grade-cap settings can
 * promote this later.
 */
export default function EnrollPhaseFormPage({ params }: PageProps) {
  const { phaseId } = use(params);
  const router = useRouter();
  const { tenantSlug, tenant } = useTenant();
  const [phases, setPhases] = useState<PublicPhase[]>([]);
  const [phaseLoadFailed, setPhaseLoadFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void fetchPublicPhases(tenantSlug)
      .then((result) => {
        if (!cancelled) setPhases(result);
      })
      .catch(() => {
        if (!cancelled) setPhaseLoadFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [tenantSlug]);

  const phase = useMemo(
    () => phases.find((item) => item.id === phaseId) ?? null,
    [phaseId, phases],
  );

  const handleSubmitted = (statusURL: string) => {
    try {
      const u = new URL(statusURL);
      router.push(`${u.pathname}?submitted=1`);
    } catch {
      globalThis.window.location.href = statusURL;
    }
  };

  return (
    <PublicEnrollmentPageShell>
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <PublicEnrollmentBrand tenant={tenant} />
        <PublicEnrollmentSteps current="form" />
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <section className="space-y-5">
          <div className="moto-content-surface rounded-3xl border p-5 shadow-sm sm:p-8">
            <PublicEnrollmentBackLink href="/enroll">
              Andere Anmeldung wählen
            </PublicEnrollmentBackLink>
            <p className="mt-6 text-sm font-semibold tracking-wide text-[#5080D8] uppercase">
              Formular ausfüllen
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight text-wrap text-gray-900 sm:text-4xl">
              {phase?.name ?? "Anmeldung"}
            </h1>
            <p className="mt-4 max-w-3xl text-base leading-7 text-gray-600">
              Bitte füllen Sie alle Pflichtfelder aus. Wenn Sie mehrere Kinder
              anmelden möchten, können Sie sie direkt in diesem Formular
              hinzufügen.
            </p>
          </div>

          <EnrollmentForm
            phaseID={phaseId}
            gradeLevelMax={4}
            onSubmitted={handleSubmitted}
          />
        </section>

        <aside className="hidden space-y-4 lg:sticky lg:top-8 lg:block lg:self-start">
          <section className="moto-content-surface rounded-3xl border p-5 shadow-sm">
            <p className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
              Ihre Anmeldung
            </p>
            <h2 className="mt-2 text-xl font-semibold text-gray-900">
              {phase?.name ?? "Anmeldeformular"}
            </h2>
            {phase ? (
              <dl className="mt-4 space-y-3 text-sm text-gray-600">
                <div>
                  <dt className="font-semibold text-gray-900">
                    Betreuungszeitraum
                  </dt>
                  <dd>
                    {formatDate(phase.service_start_date)} bis{" "}
                    {formatDate(phase.service_end_date)}
                  </dd>
                </div>
                {phase.enrollment_close_at && (
                  <div>
                    <dt className="font-semibold text-gray-900">
                      Anmeldefrist
                    </dt>
                    <dd>{formatDateTime(phase.enrollment_close_at)}</dd>
                  </div>
                )}
              </dl>
            ) : (
              <p className="mt-3 text-sm leading-6 text-gray-600">
                {phaseLoadFailed
                  ? "Die Detaildaten konnten nicht geladen werden. Das Formular kann trotzdem ausgefüllt werden."
                  : "Die Detaildaten werden geladen."}
              </p>
            )}
          </section>

          <PublicInfoCard
            icon={<CalendarDays className="h-5 w-5" />}
            title="Ein Formular pro Zeitraum"
          >
            Diese Anmeldung gilt nur für den angezeigten Betreuungszeitraum. Für
            einen anderen Zeitraum starten Sie bitte eine weitere Anmeldung.
          </PublicInfoCard>
          <PublicInfoCard icon={<Mail className="h-5 w-5" />} title="E-Mail">
            Verwenden Sie eine E-Mail-Adresse, die Sie zuverlässig erreichen.
            Dorthin gehen Rückfragen, Entscheidungen und der Status-Link.
          </PublicInfoCard>
          <PublicInfoCard
            icon={<ShieldCheck className="h-5 w-5" />}
            title="Prüfung durch die OGS"
          >
            Die OGS prüft Ihre Angaben nach dem Absenden. Eine Online-Anmeldung
            ist deshalb noch keine endgültige Platzzusage.
          </PublicInfoCard>
          <PublicInfoCard icon={<Check className="h-5 w-5" />} title="Status">
            Nach dem Absenden wechseln Sie automatisch zur Statusseite. Dort
            sehen Sie, ob die Anmeldung eingegangen ist, geprüft wird oder
            entschieden wurde.
          </PublicInfoCard>
        </aside>
      </div>
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
