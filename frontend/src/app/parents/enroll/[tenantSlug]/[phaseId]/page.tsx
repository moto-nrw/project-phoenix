"use client";

import { use } from "react";
import Link from "next/link";
// eslint-disable-next-line no-restricted-imports -- parent portal uses bare paths, no tenant-router
import { useRouter } from "next/navigation";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import { TenantProvider } from "~/components/tenant/tenant-provider";

interface PageProps {
  readonly params: Promise<{ tenantSlug: string; phaseId: string }>;
}

/**
 * Embedded enrollment form for logged-in parents. Reuses the exact
 * same EnrollmentForm component as the public per-phase page. The
 * form internally calls useTenant() — the parents subdomain has no
 * tenant context of its own, so we supply a minimal TenantProvider
 * keyed off the URL slug. tenant metadata is null here (the parents
 * shell never resolves a tenant), but the form only reads tenantSlug.
 *
 * Submit + autofill enhancements land in PR 11/3.
 */
export default function ParentEnrollFormPage({ params }: PageProps) {
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

  return (
    <main className="mx-auto max-w-3xl space-y-6 p-6">
      <header className="space-y-2">
        <Link
          href="/parents/enroll"
          className="text-sm text-[#5080D8] hover:underline"
        >
          ← Andere Schule wählen
        </Link>
        <h1 className="text-2xl font-semibold text-gray-900">Anmeldung</h1>
        <p className="text-sm text-gray-600">
          Bitte füllen Sie das Formular vollständig aus. Sie erhalten nach dem
          Absenden eine Bestätigungs-E-Mail mit einem Link, über den Sie den
          Status der Anmeldung jederzeit einsehen können.
        </p>
      </header>

      <TenantProvider tenantSlug={tenantSlug} tenant={null}>
        <EnrollmentForm
          phaseID={phaseId}
          gradeLevelMax={4}
          onSubmitted={handleSubmitted}
        />
      </TenantProvider>
    </main>
  );
}
