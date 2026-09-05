"use client";

import { useEffect, useState } from "react";
import Link from "~/components/ui/navigation-link";
import { Check, Clock, RefreshCw, UserPlus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useLocale, useTranslations } from "next-intl";
import { useTenant } from "~/lib/tenant-context";
import { formatDate } from "~/lib/date-helpers";
import { useTenantAwarePath } from "~/lib/tenant-path";
import type { TenantInfo } from "~/lib/tenant-api";
import {
  PublicEnrollmentBrand,
  PublicEnrollmentLocaleSwitcher,
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

/**
 * Phase picker for the new public landing page for parents. Lists every
 * currently-open phase as a card. Clicking a phase navigates to
 * /{tenant}/anmeldung/{phaseId} which renders the per-phase form. Was
 * previously a school-year dropdown above the form; PR B's phase
 * model splits the picker out so each phase carries its own form.
 */
export default function EnrollPhasePickerPage() {
  const t = useTranslations("enrollmentPublic");
  const locale = useLocale();
  const { tenantSlug, tenant } = useTenant();
  const tenantPath = useTenantAwarePath();
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
        const message = err instanceof Error ? err.message : t("unknownError");
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
  }, [tenantSlug, t]);

  if (loading) {
    return (
      <PublicEnrollmentPageShell withInlineSwitcher>
        <PhasePickerHeader tenant={tenant} />
        <section className="moto-content-surface flex min-h-[24rem] items-center justify-center rounded-2xl border shadow-sm">
          <div
            className="moto-content-surface rounded-xl border px-5 py-4 text-sm font-medium text-gray-600 shadow-sm"
            role="status"
            aria-live="polite"
          >
            {t("loading")}
          </div>
        </section>
      </PublicEnrollmentPageShell>
    );
  }

  return (
    <PublicEnrollmentPageShell withInlineSwitcher>
      <PhasePickerHeader tenant={tenant} />

      {error && (
        <div
          className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong mb-6 rounded-2xl border p-4 text-sm"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <div className="grid lg:grid-cols-[minmax(0,1fr)_24rem]">
          <div className="p-5 sm:p-8 lg:p-10">
            <div className="max-w-3xl">
              <p className="text-moto-blue text-sm font-semibold tracking-wide uppercase">
                {t("phaseEyebrow")}
              </p>
              <h1 className="mt-2 text-3xl font-bold tracking-tight text-wrap text-gray-900 sm:text-4xl">
                {t("phaseTitle")}
              </h1>
              <p className="mt-4 max-w-2xl text-base leading-7 text-gray-600">
                {t("phaseDescription")}
              </p>
            </div>

            <div className="mt-8">
              {!phases || phases.length === 0 ? (
                <div className="border-moto-orange/20 bg-moto-orange/10 text-moto-orange-strong rounded-xl border p-5 text-sm leading-6">
                  {enrollmentDisabled ? t("disabled") : t("noPhase")}
                </div>
              ) : (
                <ul className="space-y-3">
                  {phases.map((phase) => (
                    <li key={phase.id}>
                      <Link
                        href={tenantPath(
                          `/anmeldung/${encodeURIComponent(phase.id)}`,
                        )}
                        className="group moto-content-surface moto-hover-elevated flex flex-col gap-4 rounded-xl border p-4 text-left shadow-sm focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:flex-row sm:items-center sm:justify-between sm:p-5"
                      >
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-xs font-semibold text-gray-600">
                              {t(`kind.${phase.kind}`)}
                            </span>
                            {phase.enrollment_close_at && (
                              <span className="bg-moto-green/15 text-moto-green-strong inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold">
                                <Clock className="h-3.5 w-3.5" />
                                {t("openUntil", {
                                  date: formatDateTime(
                                    phase.enrollment_close_at,
                                    locale,
                                  ),
                                })}
                              </span>
                            )}
                            {phase.audience === "new_students" && (
                              <span className="bg-moto-blue/10 text-moto-blue-strong inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold">
                                <UserPlus className="h-3.5 w-3.5" />
                                {t("audienceNewStudents")}
                              </span>
                            )}
                            {phase.audience === "existing_students" && (
                              <span className="bg-moto-blue/10 text-moto-blue-strong inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-semibold">
                                <RefreshCw className="h-3.5 w-3.5" />
                                {t("audienceExistingStudents")}
                              </span>
                            )}
                          </div>
                          <h2 className="mt-3 text-lg font-semibold break-words text-gray-900 sm:text-xl">
                            {phase.name}
                          </h2>
                          <p className="mt-1 text-sm text-gray-600">
                            {t("serviceRange", {
                              from: formatDate(
                                phase.service_start_date,
                                false,
                                locale,
                              ),
                              to: formatDate(
                                phase.service_end_date,
                                false,
                                locale,
                              ),
                            })}
                          </p>
                        </div>
                        <span className="inline-flex h-9 w-full shrink-0 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-medium text-white transition-colors group-hover:bg-gray-800 sm:w-auto">
                          {t("start")}
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
                icon={<MotoConceptIcon concept="calendar" size={22} />}
                title={t("chooseOfferTitle")}
              >
                {t("chooseOfferText")}
              </PublicInfoCard>
              <PublicInfoCard
                icon={<Check className="h-5 w-5" />}
                title={t("submitTitle")}
              >
                {t("submitText")}
              </PublicInfoCard>
              <PublicInfoCard
                icon={<MotoConceptIcon concept="permissions" size={22} />}
                title={t("statusTitle")}
              >
                {t("statusText")}
              </PublicInfoCard>
            </div>
          </aside>
        </div>
      </section>
    </PublicEnrollmentPageShell>
  );
}

function PhasePickerHeader({ tenant }: { readonly tenant: TenantInfo | null }) {
  return (
    <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
      <PublicEnrollmentBrand tenant={tenant} />
      <div className="flex items-center gap-3">
        <PublicEnrollmentSteps current="phase" />
        <PublicEnrollmentLocaleSwitcher />
      </div>
    </div>
  );
}

function formatDateTime(value: string, locale: string): string {
  return new Date(value).toLocaleString(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
