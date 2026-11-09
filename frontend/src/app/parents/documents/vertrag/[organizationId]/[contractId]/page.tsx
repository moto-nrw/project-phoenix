"use client";

// Einen Betreuungsvertrag aus „Dokumente“ lesen und unterschreiben (#4679).
// Das Backend prüft, dass der Vertrag an die Adresse des Kontos ging und die
// Beziehung zum Kind das Unterschreiben erlaubt.

import { useTranslations } from "next-intl";
import { use } from "react";

import { ParentFamilyChildrenCard } from "~/components/parent/parent-family-children-card";
import { ContractSigningView } from "~/components/parent/care-contract-signing-view";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { parentPath } from "~/lib/parent-url";

interface PageProps {
  readonly searchParams: Promise<{ family?: string }>;
  readonly params: Promise<{ organizationId: string; contractId: string }>;
}

export default function ParentContractSigningPage({
  params,
  searchParams,
}: PageProps) {
  const { organizationId, contractId } = use(params);
  const { family } = use(searchParams);
  const familyText = useTranslations("parentFamilyChildren");
  const t = useTranslations("careContractSigning");
  return (
    <ParentPage>
      <ParentPageHeader
        title={family === "1" ? familyText("title") : t("pageTitle")}
        backHref={parentPath("/documents")}
        backLabel={t("back")}
      />
      {family === "1" ? (
        <ParentFamilyChildrenCard
          source={{ kind: "inbox", organizationId, contractId }}
        />
      ) : (
        <ContractSigningView
          source={{ kind: "inbox", organizationId, contractId }}
        />
      )}
    </ParentPage>
  );
}
