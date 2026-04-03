"use client";

import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { OperatorsManagementPage } from "~/components/operator/operators-management-page";

export default function OperatorsPage() {
  useSetBreadcrumb({ pageTitle: "Operatoren" });

  return <OperatorsManagementPage />;
}
