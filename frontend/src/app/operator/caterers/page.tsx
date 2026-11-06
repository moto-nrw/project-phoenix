"use client";

import { Suspense, useState } from "react";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { StatusBadge } from "~/components/ui/status-badge";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorCaterer } from "~/lib/operator/provisioning-helpers";

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

  const count = data?.length;

  return (
    <TenantPage
      title="Caterer"
      stats={
        count === undefined ? undefined : (
          <TenantPageStats items={[{ value: count, label: "Caterer" }]} />
        )
      }
      error={loadError}
      empty={
        !isLoading && !error && data?.length === 0
          ? {
              title: "Noch keine Caterer",
              description:
                "Noch kein Caterer hat eine Einladung einer Schule angenommen.",
            }
          : null
      }
      overlays={
        <>
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
        </>
      }
    >
      <DataTable
        columns={catererColumns(setResetTarget)}
        rows={data ?? []}
        getRowKey={(caterer) => caterer.accountId}
        defaultSortKey="name"
        isLoading={isLoading}
      />
    </TenantPage>
  );
}

export default function OperatorCaterersPage() {
  return (
    <Suspense fallback={<TenantPage title="Caterer" loading />}>
      <OperatorCaterersPageContent />
    </Suspense>
  );
}
