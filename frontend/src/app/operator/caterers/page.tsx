"use client";

import { Suspense, useMemo, useState } from "react";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { StatusBadge } from "~/components/ui/status-badge";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { mealPlanConcept } from "~/lib/concepts/mealPlan";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorCaterer } from "~/lib/operator/provisioning-helpers";
import { SimpleEmptyState } from "../provisioning/provisioning-shared";

/**
 * Die Caterer-Konten (#4497). Ein Caterer schaltet die Zwei-Faktor-Anmeldung
 * selbst ein; hat er keinen Zugriff mehr, setzt der Operator sie hier zurück.
 * Schulen können das nicht, weil ein Caterer mehrere Schulen beliefert.
 */
function catererColumns(
  onResetMfa: (caterer: OperatorCaterer) => void,
): DataTableColumn<OperatorCaterer>[] {
  return [
    {
      key: "name",
      header: "Caterer",
      render: (caterer) => (
        <span className="font-medium text-gray-900">{caterer.name}</span>
      ),
      sortValue: (caterer) => caterer.name,
    },
    {
      key: "email",
      header: "E-Mail",
      render: (caterer) => caterer.email,
      sortValue: (caterer) => caterer.email,
    },
    {
      key: "mfa",
      header: "Zwei-Faktor-Anmeldung",
      render: (caterer) => (
        <StatusBadge
          tone={caterer.mfaEnabled ? "green" : "gray"}
          label={caterer.mfaEnabled ? "An" : "Aus"}
        />
      ),
      sortValue: (caterer) => (caterer.mfaEnabled ? 1 : 0),
    },
    {
      key: "actions",
      header: "",
      // Only a switched-on second factor can be reset.
      render: (caterer) =>
        caterer.mfaEnabled ? (
          <OverflowMenu
            ariaLabel={`Aktionen für ${caterer.email}`}
            triggerSize="sm"
            items={[
              {
                label: "Zwei-Faktor zurücksetzen",
                onClick: () => onResetMfa(caterer),
              },
            ]}
          />
        ) : null,
    },
  ];
}

function OperatorCaterersPageContent() {
  useSetBreadcrumb({ pageTitle: "Caterer" });
  const [resetTarget, setResetTarget] = useState<OperatorCaterer | null>(null);

  const { data, error, isLoading, mutate } = useSWRAuth(
    operatorProvisioningKeys.caterers(),
    () => operatorProvisioningService.listCaterers(),
    { revalidateOnFocus: false },
  );
  const loadError = useSwrLoadError(
    error,
    "die Liste der Caterer",
    () => void mutate(),
  );

  const tabs = useMemo(
    () => ({
      items: [{ id: "caterers", label: "Caterer", count: data?.length }],
      activeTab: "caterers",
      onTabChange: () => undefined,
    }),
    [data?.length],
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Caterer"
        concept={mealPlanConcept}
        tabs={tabs}
      />

      <LoadErrorAlert error={loadError} className="mb-4" />

      {(data === undefined && !isLoading) || error ? null : !isLoading &&
        data?.length === 0 ? (
        <SimpleEmptyState
          title="Noch keine Caterer"
          description="Noch kein Caterer hat eine Einladung einer Schule angenommen."
        />
      ) : (
        <DataTable
          columns={catererColumns(setResetTarget)}
          rows={data ?? []}
          getRowKey={(caterer) => caterer.accountId}
          defaultSortKey="name"
          isLoading={isLoading}
        />
      )}

      {resetTarget ? (
        <MFAAdminOverrideModal
          isOpen={true}
          onClose={() => {
            setResetTarget(null);
            void mutate();
          }}
          scope="caterer"
          catererId={resetTarget.id}
          accountId={resetTarget.accountId}
          accountLabel={`${resetTarget.name} (${resetTarget.email})`}
        />
      ) : null}
    </div>
  );
}

export default function OperatorCaterersPage() {
  return (
    <Suspense>
      <OperatorCaterersPageContent />
    </Suspense>
  );
}
