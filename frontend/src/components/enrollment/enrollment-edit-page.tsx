"use client";

import { use, useCallback, useEffect, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- public/parents token flow is outside tenant-router
import { usePathname, useRouter } from "next/navigation";

import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import { prefetchedDataFromBootstrap } from "~/components/enrollment/enrollment-form-config";
import { PublicEnrollmentContentSkeleton } from "~/components/enrollment/public-enrollment-shell";
import { TenantProvider } from "~/lib/tenant-context";
import { useAnalyticsSchoolPolicy } from "~/lib/hooks/use-analytics-school-policy";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import {
  createEnrollmentChangeRequest,
  fetchEnrollmentEditBootstrap,
  updateEnrollmentRequest,
  type EnrollmentEditBootstrap,
  type SubmitEnrollmentPayload,
} from "~/lib/enrollment-submission-api";
import { createLogger } from "~/lib/logger";
import { useApiLoadError, useToast } from "~/contexts/ToastContext";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Textarea } from "~/components/ui/textarea";
import { ParentPageHeader } from "~/components/parent/parent-page";
import { announceChangeRequestsChanged } from "~/lib/change-requests-refresh";
import {
  CareFacilityTermScope,
  useCareTermTranslations,
  useProvideCareFacilityTerm,
} from "~/lib/care-facility-term";

const logger = createLogger({ component: "EnrollmentEditPage" });

interface Props {
  readonly params: Promise<{ token: string }>;
  /**
   * Reduced Halbjahreswechsel flow (#2251): only care offerings and
   * booked weekdays are editable; everything else travels unchanged.
   */
  readonly adjustOnly?: boolean;
}

/**
 * The edit form of one enrollment; its school, and with it the
 * care-facility term of the fixed texts (#6042), comes from the bootstrap.
 */
export function EnrollmentEditPage(props: Props) {
  return (
    <CareFacilityTermScope>
      <EnrollmentEditPageContent {...props} />
    </CareFacilityTermScope>
  );
}

function EnrollmentEditPageContent({ params, adjustOnly = false }: Props) {
  const { token } = use(params);
  const t = useCareTermTranslations("enrollmentStatus");
  const loadErrorObject = t("errorObject");
  const loadErrorFallback = t("editLoadError");
  const router = useRouter();
  const pathname = usePathname();
  const statusHref = pathname?.replace(/\/(edit|adjust)\/?$/, "") || ".";
  const [loaded, setLoaded] = useState<{
    token: string;
    bootstrap: EnrollmentEditBootstrap;
  } | null>(null);
  const bootstrap = loaded?.token === token ? loaded.bootstrap : null;
  useProvideCareFacilityTerm(bootstrap?.care_facility_term);
  const [changeReason, setChangeReason] = useState("");
  const [loading, setLoading] = useState(true);
  const toast = useToast();
  // A failed load stays where the form would be, with retry (#2515).
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [loadAttempt, setLoadAttempt] = useState(0);

  // The target school is unknown until bootstrap completes. Never inherit
  // an already allowed host/parent policy while loading or after a failure.
  useAnalyticsSchoolPolicy(
    "",
    !loading &&
      !loadError &&
      bootstrap !== null &&
      isSupportedGradeLevelMax(bootstrap.grade_level_max),
    `/anmeldung/status/${encodeURIComponent(token)}/${adjustOnly ? "adjust" : "edit"}`,
  );

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      clearLoadError();
      try {
        const result = await fetchEnrollmentEditBootstrap(
          token,
          loadErrorFallback,
        );
        if (!cancelled) setLoaded({ token, bootstrap: result });
      } catch (err) {
        logger.warn("enrollment_edit_bootstrap_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          void showLoadError(err, {
            object: loadErrorObject,
            retry: () => setLoadAttempt((n) => n + 1),
          });
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [
    token,
    loadErrorObject,
    loadErrorFallback,
    loadAttempt,
    clearLoadError,
    showLoadError,
  ]);

  const submitter = useCallback(
    async (payload: SubmitEnrollmentPayload) => {
      if (bootstrap?.edit_mode === "change_request") {
        const changeRequest = await createEnrollmentChangeRequest(
          token,
          payload,
          changeReason,
          t("changeRequestSaveError"),
        );
        announceChangeRequestsChanged();
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
      toast.success(
        bootstrap?.edit_mode === "change_request"
          ? t("changeRequestSent")
          : t("editSaved"),
      );
      router.push(query ? `${statusHref}?${query}` : statusHref);
    },
    [router, statusHref, toast, t, bootstrap?.edit_mode],
  );
  // The edit path re-runs the submit's eligibility gates, so the form gets
  // the same narrowed grade set (#1663).
  const prefetchedData = useMemo(
    () => (bootstrap ? prefetchedDataFromBootstrap(bootstrap) : undefined),
    [bootstrap],
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

  const isChangeRequestMode = bootstrap?.edit_mode === "change_request";
  const title = adjustOnly
    ? t("adjustTitle")
    : isChangeRequestMode
      ? t("changeRequestTitle")
      : t("editFull");

  if (loadError || !bootstrap) {
    return (
      <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:px-6 sm:py-6">
        <ParentPageHeader
          title={title}
          backHref={statusHref}
          backLabel={t("backToStatus")}
        />
        <LoadErrorAlert error={loadError ?? t("editLoadError")} />
      </main>
    );
  }

  if (!isSupportedGradeLevelMax(bootstrap.grade_level_max)) {
    return (
      <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:px-6 sm:py-6">
        <ParentPageHeader
          title={title}
          backHref={statusHref}
          backLabel={t("backToStatus")}
        />
        {/* The school allows grades this form cannot show: no API error,
            but the same dead end, so it reads like one. */}
        <LoadErrorAlert error={t("editLoadError")} />
      </main>
    );
  }

  return (
    <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:space-y-6 sm:px-6 sm:py-6">
      <ParentPageHeader
        title={title}
        description={
          adjustOnly
            ? t("adjustDescription")
            : isChangeRequestMode
              ? t("changeRequestDescription")
              : undefined
        }
        backHref={statusHref}
        backLabel={t("backToStatus")}
      />

      {isChangeRequestMode ? (
        <section className="moto-content-surface rounded-xl border p-5 sm:p-6">
          <Textarea
            label={t("changeRequestReasonLabel")}
            value={changeReason}
            onChange={(event) => setChangeReason(event.target.value)}
            rows={3}
            placeholder={t("changeRequestReasonPlaceholder")}
            className="resize-y"
          />
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
          prefetchedData={prefetchedData}
          initialDraft={bootstrap.draft}
          statusToken={token}
          submitter={submitter}
          skipCaptcha
          localizedCopy
          lockedGuardianEmail
          lockChildStructure={isChangeRequestMode || adjustOnly}
          restrictToOfferings={adjustOnly}
          withoutSignature={isChangeRequestMode}
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
