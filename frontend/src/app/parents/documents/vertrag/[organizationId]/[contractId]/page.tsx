"use client";

// Einen Betreuungsvertrag aus „Dokumente“ lesen und unterschreiben (#4679),
// die Kinderzahl der Familie angeben (#5918) oder das Lastschrift-Mandat
// erteilen (#4696). Das Backend prüft, dass der Vertrag an die Adresse des
// Kontos ging und die Beziehung zum Kind den Schritt erlaubt.

import { useTranslations } from "next-intl";
import { use } from "react";

import { ParentFamilyChildrenCard } from "~/components/parent/parent-family-children-card";
import { ContractSigningView } from "~/components/parent/care-contract-signing-view";
import { ParentSepaMandateView } from "~/components/parent/parent-sepa-mandate-view";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { parentPath } from "~/lib/parent-url";

interface PageProps {
  readonly searchParams: Promise<{ family?: string; lastschrift?: string }>;
  readonly params: Promise<{ organizationId: string; contractId: string }>;
}

export default function ParentContractSigningPage({
  params,
  searchParams,
}: PageProps) {
  const { organizationId, contractId } = use(params);
  const { family, lastschrift } = use(searchParams);
  const familyText = useTranslations("parentFamilyChildren");
  const mandateText = useTranslations("parentSepaMandate");
  const t = useTranslations("careContractSigning");
  const title =
    lastschrift === "1"
      ? mandateText("title")
      : family === "1"
        ? familyText("title")
        : t("pageTitle");
  return (
    <ParentPage>
      <ParentPageHeader
        title={title}
        backHref={parentPath("/documents")}
        backLabel={t("back")}
      />
      {lastschrift === "1" ? (
        <ParentSepaMandateView
          organizationId={organizationId}
          contractId={contractId}
        />
      ) : family === "1" ? (
        <ParentFamilyChildrenCard
          organizationId={organizationId}
          contractId={contractId}
        />
      ) : (
        <ContractSigningView
          source={{ kind: "inbox", organizationId, contractId }}
        />
      )}
    </ParentPage>
  );
}
