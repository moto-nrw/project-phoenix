"use client";

import { useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { formatCount } from "~/lib/format-utils";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OrganizationSummary } from "~/lib/operator/provisioning-helpers";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import {
  EmptyState,
  PlusIcon,
  CardSkeletons,
} from "../provisioning/provisioning-shared";
import { CreateOrganizationModal } from "../provisioning/create-organization-modal";
import {
  useSoftDeletable,
  DeletedEntityCard,
} from "../provisioning/soft-delete-shared";
import {
  OrgRestoreModal,
  OrgSoftDeleteModal,
} from "../provisioning/operator-entity-modals";

function KpiCard({ label, value }: Readonly<{ label: string; value: number }>) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white px-5 py-4 shadow-sm">
      <div className="text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {label}
      </div>
      <div className="mt-1 text-3xl font-bold text-gray-900">
        {formatCount(value)}
      </div>
    </div>
  );
}

export default function OperatorOrganizationsPage() {
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  useSetBreadcrumb({ pageTitle: "Träger" });
  const router = useRouter();

  const [createOrgOpen, setCreateOrgOpen] = useState(false);

  const {
    data: organizations,
    isLoading: orgsLoading,
    mutate: mutateOrgs,
  } = useSWR(
    isAuthenticated ? "operator-organization-summaries" : null,
    () => operatorProvisioningService.listOrganizationSummaries(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: stats, mutate: mutateStats } = useSWR(
    isAuthenticated ? "operator-provisioning-stats" : null,
    () => operatorProvisioningService.getStats(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const refreshAll = useCallback(async () => {
    await Promise.all([mutateOrgs(), mutateStats()]);
  }, [mutateOrgs, mutateStats]);

  const activeOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt == null) ?? [],
    [organizations],
  );

  const deletedOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt != null) ?? [],
    [organizations],
  );

  const orgDelete = useSoftDeletable<OrganizationSummary>({
    softDeleteFn: operatorProvisioningService.softDeleteOrganization,
    restoreFn: operatorProvisioningService.restoreOrganization,
    mutateList: mutateOrgs,
    errorMessages: {
      softDelete:
        "Fehler beim Löschen des Trägers. Bitte versuchen Sie es erneut.",
      restore:
        "Fehler beim Wiederherstellen des Trägers. Bitte versuchen Sie es erneut.",
    },
    logEventPrefix: "organization",
  });

  const orgDeleteTargetHasSchools = useMemo(() => {
    const target = orgDelete.deleteTarget;
    if (!target) return false;
    return target.schulenCount > 0;
  }, [orgDelete.deleteTarget]);

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "organizations",
          label: "Träger",
          count: organizations?.length,
        },
      ],
      activeTab: "organizations",
      onTabChange: () => undefined,
    }),
    [organizations?.length],
  );

  const actionButton = (
    <button
      type="button"
      onClick={() => setCreateOrgOpen(true)}
      className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
    >
      Neuer Träger
    </button>
  );

  const mobileActionButton = (
    <button
      type="button"
      onClick={() => setCreateOrgOpen(true)}
      className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
      aria-label="Neuer Träger"
    >
      <PlusIcon />
    </button>
  );

  const columns: DataTableColumn<OrganizationSummary>[] = useMemo(
    () => [
      {
        key: "name",
        header: "Träger",
        render: (row) => (
          <div>
            <div className="font-semibold text-gray-900">{row.name}</div>
            <div className="font-mono text-xs text-gray-500">{row.slug}</div>
          </div>
        ),
        sortValue: (row) => row.name.toLowerCase(),
      },
      {
        key: "schulen",
        header: "Schulen",
        align: "right",
        render: (row) => formatCount(row.schulenCount),
        sortValue: (row) => row.schulenCount,
      },
      {
        key: "konten",
        header: "Konten",
        align: "right",
        render: (row) => formatCount(row.kontenCount),
        sortValue: (row) => row.kontenCount,
      },
      {
        key: "geraete",
        header: "Geräte",
        align: "right",
        render: (row) => formatCount(row.geraeteCount),
        sortValue: (row) => row.geraeteCount,
      },
      {
        key: "personen",
        header: "Personen",
        align: "right",
        render: (row) => formatCount(row.personenCount),
        sortValue: (row) => row.personenCount,
      },
      {
        key: "status",
        header: "Status",
        render: (row) => <DataTableStatusBadge active={row.active} />,
        sortValue: (row) => (row.active ? 0 : 1),
      },
    ],
    [],
  );

  const handleRowClick = useCallback(
    (row: OrganizationSummary) => {
      router.push(`/operator/organizations/${encodeURIComponent(row.slug)}`);
    },
    [router],
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Träger"
        concept="organizations"
        tabs={tabs}
        actionButton={actionButton}
        mobileActionButton={mobileActionButton}
      />

      {stats ? (
        <div className="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
          <KpiCard label="Träger" value={stats.traegerCount} />
          <KpiCard label="Schulen" value={stats.schulenCount} />
          <KpiCard label="Konten" value={stats.kontenCount} />
          <KpiCard label="Geräte" value={stats.geraeteCount} />
        </div>
      ) : null}

      {orgsLoading && <CardSkeletons />}

      {!orgsLoading && (
        <div className="mt-6">
          {deletedOrganizations.length > 0 && (
            <div className="mb-4 flex justify-end">
              <button
                type="button"
                onClick={() => orgDelete.setShowTrash(!orgDelete.showTrash)}
                className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  orgDelete.showTrash
                    ? "bg-moto-red/15 text-moto-red-strong hover:bg-moto-red/20"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                }`}
              >
                Papierkorb ({deletedOrganizations.length})
              </button>
            </div>
          )}

          {orgDelete.showTrash ? (
            <div className="mt-4 space-y-4">
              {deletedOrganizations.map((org) => (
                <DeletedEntityCard
                  key={org.id}
                  name={org.name}
                  subtitle={org.slug}
                  deletedAt={org.deletedAt}
                  onRestore={() => orgDelete.setRestoreTarget(org)}
                />
              ))}
            </div>
          ) : activeOrganizations.length === 0 ? (
            <EmptyState
              title="Keine Träger"
              description="Erstellen Sie einen neuen Träger, um Schulen zu verwalten."
              buttonLabel="Neuer Träger"
              onAction={() => setCreateOrgOpen(true)}
            />
          ) : (
            <DataTable
              columns={columns}
              rows={activeOrganizations}
              getRowKey={(row) => row.id}
              onRowClick={handleRowClick}
              defaultSortKey="name"
            />
          )}
        </div>
      )}

      {orgDelete.deleteTarget && (
        <OrgSoftDeleteModal
          target={orgDelete.deleteTarget}
          inputId="delete-org-confirm"
          confirmInput={orgDelete.deleteConfirmInput}
          onConfirmInputChange={orgDelete.setDeleteConfirmInput}
          errorMessage={orgDelete.softDeleteError}
          isProcessing={orgDelete.isProcessing}
          onCancel={() => orgDelete.setDeleteTarget(null)}
          onConfirm={() => void orgDelete.handleSoftDelete()}
          confirmDisabled={orgDeleteTargetHasSchools}
          confirmDisabledReason="Dieser Träger hat noch nicht gelöschte Schulen. Bitte zuerst alle Schulen löschen."
        />
      )}

      <OrgRestoreModal
        target={orgDelete.restoreTarget}
        setTarget={orgDelete.setRestoreTarget}
        extraMessage="Gelöschte Schulen dieses Trägers müssen separat wiederhergestellt werden."
        onConfirm={() => void orgDelete.handleRestore()}
        isProcessing={orgDelete.isProcessing}
        errorMessage={orgDelete.softDeleteError}
      />

      <CreateOrganizationModal
        isOpen={createOrgOpen}
        onClose={() => setCreateOrgOpen(false)}
        onCreated={() => refreshAll().then(() => undefined)}
      />
    </div>
  );
}
