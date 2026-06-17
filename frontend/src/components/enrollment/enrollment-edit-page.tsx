"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
// eslint-disable-next-line no-restricted-imports -- public/parents token flow is outside tenant-router
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import { TenantProvider } from "~/components/tenant/tenant-provider";
import {
  fetchEnrollmentEditBootstrap,
  updateEnrollmentRequest,
  type EnrollmentEditBootstrap,
  type SubmitEnrollmentPayload,
} from "~/lib/enrollment-submission-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentEditPage" });

interface Props {
  readonly params: Promise<{ token: string }>;
}

export function EnrollmentEditPage({ params }: Props) {
  const { token } = use(params);
  const t = useTranslations("enrollmentStatus");
  const router = useRouter();
  const pathname = usePathname();
  const statusHref = pathname?.replace(/\/edit\/?$/, "") || ".";
  const [bootstrap, setBootstrap] = useState<EnrollmentEditBootstrap | null>(
    null,
  );
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
    (payload: SubmitEnrollmentPayload) =>
      updateEnrollmentRequest(token, payload, t("editSaveError")),
    [token, t],
  );

  const handleSubmitted = useCallback(() => {
    router.push(statusHref);
  }, [router, statusHref]);

  if (loading) {
    return (
      <main className="mx-auto w-full max-w-4xl px-4 py-5 sm:px-6 sm:py-6">
        <div className="moto-content-surface rounded-xl border p-5 text-sm font-medium text-gray-600 shadow-sm sm:p-6">
          {t("editLoading")}
        </div>
      </main>
    );
  }

  if (error || !bootstrap) {
    return (
      <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:px-6 sm:py-6">
        <Link
          href={statusHref}
          className="inline-flex items-center gap-2 text-sm font-semibold text-[#5080D8] hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("backToStatus")}
        </Link>
        <div
          role="alert"
          className="rounded-2xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-5 text-sm text-[#CC2626] shadow-sm"
        >
          {error ?? t("editLoadError")}
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto w-full max-w-4xl space-y-5 px-4 py-5 sm:space-y-6 sm:px-6 sm:py-6">
      <header className="space-y-2">
        <Link
          href={statusHref}
          className="inline-flex items-center gap-2 text-sm font-semibold text-[#5080D8] hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          {t("backToStatus")}
        </Link>
        <h1 className="text-2xl font-semibold text-wrap text-gray-900">
          {t("editFull")}
        </h1>
      </header>

      <TenantProvider tenantSlug={bootstrap.draft.tenant_slug} tenant={null}>
        <EnrollmentForm
          phaseID={bootstrap.draft.phase_id}
          gradeLevelMax={4}
          onSubmitted={handleSubmitted}
          prefetchedData={{
            schema: bootstrap.schema,
            offerings: bootstrap.offerings,
            careOfferingSelectionMode: bootstrap.care_offering_selection_mode,
            captchaConfig: null,
            legalTexts: bootstrap.legal_texts,
            profile: null,
          }}
          initialDraft={bootstrap.draft}
          submitter={submitter}
          skipCaptcha
          localizedCopy
          lockedGuardianEmail
          submitLabel={t("editSubmit")}
        />
      </TenantProvider>
    </main>
  );
}
