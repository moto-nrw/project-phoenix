"use client";

// Öffentliche Seite hinter dem einmaligen Link aus der Vertragsmail (#4679).
// Eltern brauchen kein Konto: Der Link ist die Berechtigung, er nennt genau
// einen Vertrag. Lesen verbraucht ihn nicht; die Unterschrift schließt ihn.

import { useTranslations } from "next-intl";
import { use } from "react";

import { ContractSigningView } from "~/components/parent/care-contract-signing-view";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";
import { useAnalyticsSchoolPolicy } from "~/lib/hooks/use-analytics-school-policy";

interface PageProps {
  readonly params: Promise<{ link: string }>;
}

export default function CareContractSigningPage({ params }: PageProps) {
  // The link names a Träger, no school: no school policy permits collection.
  useAnalyticsSchoolPolicy("", false);
  const { link } = use(params);
  const t = useTranslations("careContractSigning");
  return (
    <PublicEnrollmentPageShell>
      <div className="mx-auto w-full max-w-3xl space-y-5">
        <h1 className="sr-only">{t("pageTitle")}</h1>
        <ContractSigningView source={{ kind: "link", link }} />
      </div>
    </PublicEnrollmentPageShell>
  );
}
