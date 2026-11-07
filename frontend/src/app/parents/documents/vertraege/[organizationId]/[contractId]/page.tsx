"use client";

import { use } from "react";
import { ParentContractNoticePage } from "~/components/parent/parent-contract-notice-page";

// Vertrag kündigen (#4682): die Seite hinter „Verträge hier kündigen“.
export default function ParentsContractNoticePage({
  params,
}: Readonly<{
  params: Promise<{ organizationId: string; contractId: string }>;
}>) {
  const { organizationId, contractId } = use(params);
  return (
    <ParentContractNoticePage
      organizationId={organizationId}
      contractId={contractId}
    />
  );
}
