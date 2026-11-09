"use client";

import { use } from "react";
import { ParentContractNoticePage } from "~/components/parent/parent-contract-notice-page";

// Vertrag kündigen (#4682): die Seite hinter „Verträge hier kündigen“. Mit
// `?widerruf=1` öffnet sie der Knopf „Vertrag widerrufen“ (§ 356a BGB,
// #6105) mit dem Widerruf vorgewählt.
export default function ParentsContractNoticePage({
  params,
  searchParams,
}: Readonly<{
  params: Promise<{ organizationId: string; contractId: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}>) {
  const { organizationId, contractId } = use(params);
  const { widerruf } = use(searchParams);
  return (
    <ParentContractNoticePage
      organizationId={organizationId}
      contractId={contractId}
      withdrawal={widerruf === "1"}
    />
  );
}
