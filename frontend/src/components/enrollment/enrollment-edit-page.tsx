"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "~/components/ui/navigation-link";
// eslint-disable-next-line no-restricted-imports -- public/parents token flow is outside tenant-router
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import { PublicEnrollmentContentSkeleton } from "~/components/enrollment/public-enrollment-shell";
import { TenantProvider } from "~/lib/tenant-context";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import {
  createEnrollmentChangeRequest,
  fetchEnrollmentEditBootstrap,
  updateEnrollmentRequest,
  type EnrollmentEditBootstrap,
  type SubmitEnrollmentPayload,
} from "~/lib/enrollment-submission-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentEditPage" });

interface Props {
  readonly params: Promise<{ token: string }>;
  /**
   * Reduced Halbjahreswechsel flow (#2251): only care offerings and
   * booked weekdays are editable; everything else travels unchanged.
   */
  readonly adjustOnly?: boolean;
}

export function EnrollmentEditPage({ params, adjustOnly = false }: Props) {
  const { token } = use(params);
  const t = useTranslations("enrollmentStatus");
  const router = useRouter();
  const pathname = usePathname();
  const statusHref = pathname?.replace(/\/(edit|adjust)\/?$/, "") || ".";
  const [bootstrap, setBootstrap] = useState<EnrollmentEditBootstrap | null>(
    null,
  );
  const [changeReason, setChangeReason] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const result = await fetchEnrollmentEditBootstrap(
          token,
          t("editLoadError"),
        );
        if (!cancelled) setBootstrap(result);
      } catch (err) {
        const message = err instanceof Error ? err.message : t("editLoadError");
        logger.warn("enrollment_edit_bootstrap_failed", { error: message });
        if (!cancelled) setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [token, t]);

  const submitter = useCallback(
    async (payload: SubmitEnrollmentPayload) => {
      if (bootstrap?.edit_mode === "change_request") {
        const changeRequest = await createEnrollmentChangeRequest(
          token,
          payload,
          changeReason,
          t("changeRequestSaveError"),
        );
        window.dispatchEvent(new Event("change-requests-refresh"));
        return {
          request_id: changeRequest.request_id,
          status_url: statusHref,
        };
      }
      return updateEnrollmentRequest(token, payload, t("editSaveError"));
    },
    [bootstrap?.edit_mode, changeReason, statusHref, token, t],
  );

  const handleSubmitted = useCallback(
    (statusURL: string) => {
      const submittedURL = new URL(statusURL, globalThis.location.origin);
      const query = submittedURL.searchParams.toString();
      router.push(query ? `${statusHref}?${query}` : statusHref);
    },
    [router, statusHref],
  );

  if (loading) {
    return (
      <main className="mx-auto w-full max-w-4xl px-4 py-5 sm:px-6 sm:py-6">
        <span role="status" className="sr-only">
          {t("editLoading")}
        </span>
        <PublicEnrollmentContentSkeleton sections={3} />
      </main>
    );
  }

  if (error || !bootstrap) {
    return (
      <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:px-6 sm:py-6">
        <Link
          href={statusHref}
          className="text-moto-blue inline-flex items-center gap-2 text-sm font-semibold hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("backToStatus")}
        </Link>
        <div
          role="alert"
          className="border-moto-red/30 bg-moto-red/5 text-moto-red-strong rounded-2xl border p-5 text-sm shadow-sm"
        >
          {error ?? t("editLoadError")}
        </div>
      </main>
    );
  }

  const isChangeRequestMode = bootstrap.edit_mode === "change_request";
  if (!isSupportedGradeLevelMax(bootstrap.grade_level_max)) {
    return (
      <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:px-6 sm:py-6">
        <Link
          href={statusHref}
          className="text-moto-blue inline-flex items-center gap-2 text-sm font-semibold hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("backToStatus")}
        </Link>
        <div
          role="alert"
          className="border-moto-red/30 bg-moto-red/5 text-moto-red-strong rounded-2xl border p-5 text-sm shadow-sm"
        >
          {t("editLoadError")}
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:space-y-6 sm:px-6 sm:py-6">
      <header className="space-y-2">
        <Link
          href={statusHref}
          className="text-moto-blue inline-flex items-center gap-2 text-sm font-semibold hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("backToStatus")}
        </Link>
        <h1 className="text-2xl font-semibold text-wrap text-gray-900">
          {adjustOnly
            ? t("adjustTitle")
            : isChangeRequestMode
              ? t("changeRequestTitle")
              : t("editFull")}
        </h1>
        {adjustOnly ? (
          <p className="max-w-3xl text-sm leading-6 text-gray-600">
            {t("adjustDescription")}
          </p>
        ) : isChangeRequestMode ? (
          <p className="max-w-3xl text-sm leading-6 text-gray-600">
            {t("changeRequestDescription")}
          </p>
        ) : null}
      </header>

      {isChangeRequestMode ? (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            {t("changeRequestEyebrow")}
          </p>
          <label className="mt-3 block">
            <span className="text-sm font-semibold text-gray-700">
              {t("changeRequestReasonLabel")}
            </span>
            <textarea
              value={changeReason}
              onChange={(event) => setChangeReason(event.target.value)}
              rows={3}
              placeholder={t("changeRequestReasonPlaceholder")}
              className="mt-2 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
            />
          </label>
        </section>
      ) : null}

      <TenantProvider
        tenantSlug={bootstrap.draft.tenant_subdomain}
        tenant={null}
      >
        <EnrollmentForm
          phaseID={bootstrap.draft.phase_id}
          gradeLevelMax={bootstrap.grade_level_max}
          onSubmitted={handleSubmitted}
          prefetchedData={{
            schema: bootstrap.schema,
            offerings: bootstrap.offerings,
            careOfferingSelectionMode: bootstrap.care_offering_selection_mode,
            collectGradeLevel: bootstrap.collect_grade_level,
            careOfferingsEnabled: bootstrap.care_offerings_enabled,
            captchaConfig: null,
            legalTexts: bootstrap.legal_texts,
            profile: null,
            schoolClass: bootstrap.school_class,
            // The edit path re-runs the same eligibility gates as submit, so
            // the grade select must offer the same narrowed set (#1663).
            eligibleGradeLevels: bootstrap.phase.eligible_grade_levels ?? [],
          }}
          initialDraft={bootstrap.draft}
          submitter={submitter}
          skipCaptcha
          localizedCopy
          lockedGuardianEmail
          lockChildStructure={isChangeRequestMode || adjustOnly}
          restrictToOfferings={adjustOnly}
          submitLabel={
            adjustOnly
              ? t("adjustSubmit")
              : isChangeRequestMode
                ? t("changeRequestSubmit")
                : t("editSubmit")
          }
        />
      </TenantProvider>
    </main>
  );
}
