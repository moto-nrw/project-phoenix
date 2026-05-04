"use client";

import { use } from "react";
// eslint-disable-next-line no-restricted-imports -- public page; tenant-router not needed
import { useRouter } from "next/navigation";
import Link from "next/link";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";

interface PageProps {
  readonly params: Promise<{ tenant: string; phaseId: string }>;
}

/**
 * Per-phase enrollment form page. The parent picks a phase on the
 * landing page (`/enroll`); this page renders the form scoped to that
 * phase. The form fetches its own schema + offerings using the phaseId,
 * and posts to /api/enrollment/{tenantSlug}/submit with phase_id set.
 *
 * Grade-level cap is hardcoded to 4 (OGS default) — same as the
 * pre-phase page. PR D autofill or per-phase grade-cap settings can
 * promote this later.
 */
export default function EnrollPhaseFormPage({ params }: PageProps) {
  const { phaseId } = use(params);
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
        <Link href="/enroll" className="text-sm text-[#5080D8] hover:underline">
          ← Andere Phase wählen
        </Link>
        <h1 className="text-2xl font-semibold text-gray-900">Anmeldung</h1>
        <p className="text-sm text-gray-600">
          Bitte fülle das Formular vollständig aus. Du erhältst nach dem
          Absenden eine Bestätigungs-E-Mail mit einem Link, über den du den
          Status deiner Anmeldung jederzeit einsehen kannst.
        </p>
      </header>

      <EnrollmentForm
        phaseID={phaseId}
        gradeLevelMax={4}
        onSubmitted={handleSubmitted}
      />
    </main>
  );
}
