"use client";

import { Suspense, use, useEffect, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- public page; tenant-router not needed
import { useRouter, useSearchParams } from "next/navigation";
import { Check, Mail } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useLocale, useTranslations } from "next-intl";
import {
  EnrollmentForm,
  type EnrollmentFormPrefetchedData,
} from "~/components/enrollment/enrollment-form";
import {
  PublicEnrollmentBackLink,
  PublicEnrollmentBrand,
  PublicEnrollmentLocaleSwitcher,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
  PublicInfoCard,
} from "~/components/enrollment/public-enrollment-shell";
import { useTenant } from "~/lib/tenant-context";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import {
  fetchPublicEnrollmentBootstrap,
  type PublicEnrollmentBootstrap,
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
 * The grade-level cap comes from the tenant resolve contract so the client
 * enforces the same tenant setting as submission validation.
 */
export default function EnrollPhaseFormPage({ params }: PageProps) {
  return (
    <Suspense fallback={null}>
      <EnrollPhaseFormPageContent params={params} />
    </Suspense>
  );
}

function EnrollPhaseFormPageContent({ params }: PageProps) {
  const t = useTranslations("enrollmentPublic");
  const locale = useLocale();
  const { phaseId } = use(params);
  const router = useRouter();
  const searchParams = useSearchParams();
  const lateInviteToken = searchParams.get("late_invite")?.trim();
  const { tenantSlug, tenant } = useTenant();
  const [bootstrap, setBootstrap] = useState<PublicEnrollmentBootstrap | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    void fetchPublicEnrollmentBootstrap(tenantSlug, phaseId, {
      lateInviteToken,
    })
      .then((result) => {
        if (!cancelled) setBootstrap(result);
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : t("unknownError"));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [lateInviteToken, phaseId, tenantSlug, t]);

  const phase = bootstrap?.phase ?? null;
  const resolvedGradeLevelMax = tenant?.gradeLevelMax;
  const gradeLevelMax = isSupportedGradeLevelMax(resolvedGradeLevelMax)
    ? resolvedGradeLevelMax
    : null;
  const prefetchedData = useMemo<
    EnrollmentFormPrefetchedData | undefined
  >(() => {
    if (!bootstrap) return undefined;
    return {
      schema: bootstrap.schema,
      offerings: bootstrap.offerings,
      careOfferingSelectionMode: bootstrap.care_offering_selection_mode,
      collectGradeLevel: bootstrap.collect_grade_level,
      careOfferingsEnabled: bootstrap.care_offerings_enabled,
      captchaConfig: bootstrap.captcha_config,
      legalTexts: bootstrap.legal_texts,
      profile: bootstrap.profile ?? null,
      lateInvite: bootstrap.late_invite,
      schoolClass: bootstrap.school_class,
      // Carry the phase audience through so the form can suppress the
      // existing-child reuse panel on a new_students phase — a signed-in
      // dual-role guardian on the tenant-public path can have linked
      // children the server would always reject (#1663). Mirrors the
      // parent-portal mapper.
      audience: bootstrap.phase.audience,
      // Same for the grade restriction, so the grade select offers only the
      // grades the phase accepts (#1663).
      eligibleGradeLevels: bootstrap.phase.eligible_grade_levels ?? [],
    };
  }, [bootstrap]);

  const handleSubmitted = (statusURL: string) => {
    try {
      const u = new URL(statusURL);
      u.searchParams.set("submitted", "1");
      router.push(`${u.pathname}?${u.searchParams.toString()}`);
    } catch {
      globalThis.window.location.href = statusURL;
    }
  };

  return (
    <PublicEnrollmentPageShell withInlineSwitcher>
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <PublicEnrollmentBrand tenant={tenant} />
        <div className="flex items-center gap-3">
          <PublicEnrollmentSteps current="form" />
          <PublicEnrollmentLocaleSwitcher />
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <section className="space-y-5">
          <div className="moto-content-surface rounded-2xl border p-5 shadow-sm sm:p-8">
            <PublicEnrollmentBackLink href="/enroll">
              {t("backToPicker")}
            </PublicEnrollmentBackLink>
            <p className="text-moto-blue mt-6 text-sm font-semibold tracking-wide uppercase">
              {t("formEyebrow")}
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight text-wrap text-gray-900 sm:text-4xl">
              {phase?.name ?? t("formTitleFallback")}
            </h1>
            <p className="mt-4 max-w-3xl text-base leading-7 text-gray-600">
              {t("formDescription")}
            </p>
          </div>

          {loading ? (
            <div className="moto-content-surface rounded-2xl border p-6 text-sm font-medium text-gray-600 shadow-sm">
              {t("detailsLoading")}
            </div>
          ) : error || gradeLevelMax === null ? (
            <div className="moto-content-surface border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-6 text-sm font-medium shadow-sm">
              {error ?? t("detailsLoadFailed")}
            </div>
          ) : (
            <EnrollmentForm
              phaseID={phaseId}
              gradeLevelMax={gradeLevelMax}
              onSubmitted={handleSubmitted}
              prefetchedData={prefetchedData}
              lateInviteToken={lateInviteToken ?? undefined}
              localizedCopy
            />
          )}
        </section>

        <aside className="hidden space-y-4 lg:sticky lg:top-8 lg:block lg:self-start">
          <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
            <p className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
              {t("sideTitleFallback")}
            </p>
            <h2 className="mt-2 text-xl font-semibold text-gray-900">
              {phase?.name ?? t("sideTitleFallback")}
            </h2>
            {phase ? (
              <dl className="mt-4 space-y-3 text-sm text-gray-600">
                <div>
                  <dt className="font-semibold text-gray-900">
                    {t("carePeriod")}
                  </dt>
                  <dd>
                    {t("dateRange", {
                      from: formatDate(phase.service_start_date, locale),
                      to: formatDate(phase.service_end_date, locale),
                    })}
                  </dd>
                </div>
                {phase.enrollment_close_at && (
                  <div>
                    <dt className="font-semibold text-gray-900">
                      {t("deadline")}
                    </dt>
                    <dd>{formatDateTime(phase.enrollment_close_at, locale)}</dd>
                  </div>
                )}
              </dl>
            ) : (
              <p className="mt-3 text-sm leading-6 text-gray-600">
                {error ? t("detailsLoadFailed") : t("detailsLoading")}
              </p>
            )}
          </section>

          <PublicInfoCard
            icon={<MotoConceptIcon concept="calendar" size={22} />}
            title={t("oneFormTitle")}
          >
            {t("oneFormText")}
          </PublicInfoCard>
          <PublicInfoCard
            icon={<Mail className="h-5 w-5" />}
            title={t("emailTitle")}
          >
            {t("emailText")}
          </PublicInfoCard>
          <PublicInfoCard
            icon={<MotoConceptIcon concept="permissions" size={22} />}
            title={t("reviewTitle")}
          >
            {t("reviewText")}
          </PublicInfoCard>
          <PublicInfoCard
            icon={<Check className="h-5 w-5" />}
            title={t("statusTitle")}
          >
            {t("statusAfterSubmitText")}
          </PublicInfoCard>
        </aside>
      </div>
    </PublicEnrollmentPageShell>
  );
}

function formatDate(value: string, locale: string): string {
  return new Date(value).toLocaleDateString(locale, {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function formatDateTime(value: string, locale: string): string {
  return new Date(value).toLocaleString(locale, {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
