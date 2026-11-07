"use client";

// Einen Betreuungsvertrag aus „Dokumente“ lesen und unterschreiben (#4679).
// Das Backend prüft, dass der Vertrag an die Adresse des Kontos ging und die
// Beziehung zum Kind das Unterschreiben erlaubt.

import { useTranslations } from "next-intl";
import { use } from "react";

import { ContractSigningView } from "~/components/parent/care-contract-signing-view";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { parentPath } from "~/lib/parent-url";

interface PageProps {
  readonly params: Promise<{ organizationId: string; contractId: string }>;
}

export default function ParentContractSigningPage({ params }: PageProps) {
  const { organizationId, contractId } = use(params);
  const t = useTranslations("careContractSigning");
  return (
    <ParentPage>
      <ParentPageHeader
        title={t("pageTitle")}
        backHref={parentPath("/documents")}
        backLabel={t("back")}
      />
      <ContractSigningView
        source={{ kind: "inbox", organizationId, contractId }}
      />
    </ParentPage>
  );
}
