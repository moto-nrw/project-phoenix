"use client";

import { use, useCallback } from "react";
import Link from "next/link";
// eslint-disable-next-line no-restricted-imports -- parent portal uses bare paths, no tenant-router
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import { TenantProvider } from "~/components/tenant/tenant-provider";
import {
  fetchParentEnrollmentProfile,
  submitParentEnrollment,
} from "~/lib/parent-api";

interface PageProps {
  readonly params: Promise<{ tenantSlug: string; phaseId: string }>;
}

/**
 * Embedded enrollment form for logged-in parents. Reuses the exact
 * same EnrollmentForm component as the public per-phase page. The
 * form internally calls useTenant(), but the parents subdomain has no
 * tenant context of its own, so we supply a minimal TenantProvider
 * keyed off the URL slug. tenant metadata is null here (the parents
 * shell never resolves a tenant), but the form only reads tenantSlug.
 *
 * Submit + autofill enhancements land in PR 11/3.
 */
export default function ParentEnrollFormPage({ params }: PageProps) {
  const t = useTranslations("enrollmentPublic");
  const { tenantSlug, phaseId } = use(params);
  const router = useRouter();

  const handleSubmitted = (statusURL: string) => {
    try {
      const u = new URL(statusURL);
      router.push(`${u.pathname}?submitted=1`);
    } catch {
      globalThis.window.location.href = statusURL;
    }
  };

  const profileFetcher = useCallback(
    () => fetchParentEnrollmentProfile(tenantSlug),
    [tenantSlug],
  );
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
          className="inline-flex items-center gap-2 text-sm font-semibold text-[#5080D8] hover:underline focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
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

      <TenantProvider tenantSlug={tenantSlug} tenant={null}>
        <EnrollmentForm
          phaseID={phaseId}
          gradeLevelMax={4}
          onSubmitted={handleSubmitted}
          profileFetcher={profileFetcher}
          submitter={submitter}
          skipCaptcha
          localizedCopy
        />
      </TenantProvider>
    </main>
  );
}
