"use client";

import { Suspense, use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
// eslint-disable-next-line no-restricted-imports -- parent portal uses bare paths, no tenant-router
import { useRouter, useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import {
  EnrollmentForm,
  type EnrollmentFormPrefetchedData,
} from "~/components/enrollment/enrollment-form";
import { TenantProvider } from "~/lib/tenant-context";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import {
  fetchParentEnrollmentProfile,
  submitParentEnrollment,
} from "~/lib/parent-api";
import {
  fetchParentEnrollmentBootstrap,
  type PublicEnrollmentBootstrap,
} from "~/lib/enrollment-submission-api";
import { resolveTenant, type TenantInfo } from "~/lib/tenant-api";

interface PageProps {
  readonly params: Promise<{ tenantSlug: string; phaseId: string }>;
}

/**
 * Embedded enrollment form for logged-in parents. Reuses the exact
 * same EnrollmentForm component as the public per-phase page. The
 * form internally calls useTenant(), but the parents subdomain has no tenant
 * context of its own. Resolve the public tenant metadata explicitly and pass
 * it through a local TenantProvider; parent profile and submit traffic remain
 * on the parent-authenticated endpoints.
 */
export default function ParentEnrollFormPage({ params }: PageProps) {
  return (
    <Suspense fallback={null}>
      <ParentEnrollFormPageContent params={params} />
    </Suspense>
  );
}

function ParentEnrollFormPageContent({ params }: PageProps) {
  const t = useTranslations("enrollmentPublic");
  const { tenantSlug, phaseId } = use(params);
  const router = useRouter();
  const searchParams = useSearchParams();
  const lateInviteToken = searchParams.get("late_invite")?.trim();
  const [loaded, setLoaded] = useState<{
    tenant: TenantInfo;
    prefetchedData: EnrollmentFormPrefetchedData;
  } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    setLoaded(null);

    void Promise.all([
      resolveTenant(tenantSlug),
      // Authenticated parent bootstrap (#1663): the anonymous public path
      // now refuses audience-restricted (linked_parents) phases, so the
      // parents portal loads form metadata through its own parent-scoped
      // endpoint. Parent autofill still comes only from the profile call
      // below.
      fetchParentEnrollmentBootstrap(tenantSlug, phaseId, {
        lateInviteToken,
      }),
      // Parent-profile lookup is best-effort, matching EnrollmentForm's
      // existing autofill contract. It stays on the parent-authenticated API;
      // public form metadata never receives the parent session token.
      fetchParentEnrollmentProfile(tenantSlug).catch(() => null),
    ])
      .then(([tenant, bootstrap, profile]) => {
        if (cancelled) return;
        if (!tenant || !isSupportedGradeLevelMax(tenant.gradeLevelMax)) {
          setError(t("detailsLoadFailed"));
          return;
        }
        setLoaded({
          tenant,
          prefetchedData: toParentPrefetchedData(bootstrap, profile),
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : t("unknownError"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [lateInviteToken, phaseId, tenantSlug, t]);

  const handleSubmitted = (statusURL: string) => {
    try {
      const u = new URL(statusURL);
      u.searchParams.set("submitted", "1");
      router.push(`${u.pathname}?${u.searchParams.toString()}`);
    } catch {
      globalThis.window.location.href = statusURL;
    }
  };

  const submitter = useCallback(
    (payload: Parameters<typeof submitParentEnrollment>[1]) =>
      submitParentEnrollment(tenantSlug, payload),
    [tenantSlug],
  );

  return (
    <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:space-y-6 sm:px-6 sm:py-6">
      <header className="space-y-2">
        <Link
          href="/parents"
          className="text-moto-blue inline-flex items-center gap-2 text-sm font-semibold hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("backToPortal")}
        </Link>
        <h1 className="text-2xl font-semibold text-wrap text-gray-900">
          {t("formEyebrow")}
        </h1>
        <p className="max-w-3xl text-sm leading-6 text-gray-600">
          {t("parentEmbeddedDescription")}
        </p>
      </header>

      {loading ? (
        <div className="moto-content-surface rounded-xl border p-5 text-sm font-medium text-gray-600 shadow-sm sm:p-6">
          {t("detailsLoading")}
        </div>
      ) : error || !loaded ? (
        <div
          role="alert"
          className="border-moto-red/30 bg-moto-red/5 text-moto-red-strong rounded-2xl border p-5 text-sm shadow-sm"
        >
          {error ?? t("detailsLoadFailed")}
        </div>
      ) : (
        <TenantProvider tenantSlug={tenantSlug} tenant={loaded.tenant}>
          <EnrollmentForm
            phaseID={phaseId}
            gradeLevelMax={loaded.tenant.gradeLevelMax}
            onSubmitted={handleSubmitted}
            prefetchedData={loaded.prefetchedData}
            submitter={submitter}
            lateInviteToken={lateInviteToken ?? undefined}
            skipCaptcha
            localizedCopy
          />
        </TenantProvider>
      )}
    </main>
  );
}

function toParentPrefetchedData(
  bootstrap: PublicEnrollmentBootstrap,
  profile: Awaited<ReturnType<typeof fetchParentEnrollmentProfile>>,
): EnrollmentFormPrefetchedData {
  return {
    schema: bootstrap.schema,
    offerings: bootstrap.offerings,
    careOfferingSelectionMode: bootstrap.care_offering_selection_mode,
    collectGradeLevel: bootstrap.collect_grade_level,
    careOfferingsEnabled: bootstrap.care_offerings_enabled,
    captchaConfig: null,
    legalTexts: bootstrap.legal_texts,
    // Ignore any profile on the shared bootstrap shape. Parent autofill stays
    // scoped to the parent API, independent of public metadata transport.
    profile,
    schoolClass: bootstrap.school_class,
    // Carry the phase audience so the form can suppress linked-child reuse
    // on a new_students phase instead of letting the parent adopt an
    // already-enrolled child the server would reject (#1663).
    audience: bootstrap.phase.audience,
    eligibleGradeLevels: bootstrap.phase.eligible_grade_levels ?? [],
  };
}
