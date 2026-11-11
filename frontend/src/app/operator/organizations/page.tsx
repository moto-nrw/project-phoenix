"use client";

import { useAccessControlledData } from "~/lib/hooks/use-access-controlled-data";
import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";

import { useCallback, useLayoutEffect, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { useSWRAuth } from "~/lib/swr";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { StatCard } from "~/components/ui/stat-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  STALE_DATA_SUFFIX,
  useRefreshAfterWrite,
} from "~/lib/hooks/use-refresh-after-write";
import { formatCount } from "~/lib/format-utils";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OrganizationSummary } from "~/lib/operator/provisioning-helpers";
import { Button } from "~/components/ui/button";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { ProvisioningEmptyState } from "../provisioning/provisioning-shared";
import { CreateOrganizationModal } from "../provisioning/create-organization-modal";
import { CreateTestTraegerModal } from "../provisioning/create-test-traeger-modal";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useSoftDeletable,
  DeletedEntityCard,
  TrashToggleButton,
} from "../provisioning/soft-delete-shared";
import {
  OrgRestoreModal,
  OrgSoftDeleteModal,
} from "../provisioning/operator-entity-modals";

export default function OperatorOrganizationsPage() {
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  useSetBreadcrumb({ pageTitle: "Träger" });
  const router = useRouter();

  const [createOrgOpen, setCreateOrgOpen] = useState(false);
  const [createTestOpen, setCreateTestOpen] = useState(false);

  const {
    data: cachedOrganizations,
    error: orgsError,
    isLoading: orgsLoading,
    mutate: mutateOrgs,
  } = useSWRAuth(
    isAuthenticated ? operatorProvisioningKeys.organizationSummaries() : null,
    () => operatorProvisioningService.listOrganizationSummaries(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );
  const { data: organizations, accessRefused: organizationsAccessRefused } =
    useAccessControlledData(
      isAuthenticated ? operatorProvisioningKeys.organizationSummaries() : null,
      cachedOrganizations,
      orgsError,
    );

  const {
    data: cachedStats,
    error: statsError,
    mutate: mutateStats,
  } = useSWRAuth(
    isAuthenticated ? operatorProvisioningKeys.provisioningStats() : null,
    () => operatorProvisioningService.getStats(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );
  const { data: stats } = useAccessControlledData(
    isAuthenticated ? operatorProvisioningKeys.provisioningStats() : null,
    cachedStats,
    statsError,
  );
  const orgsLoadError = useSwrLoadError(
    orgsError,
    "die Liste der Träger",
    () => void mutateOrgs(),
    organizations?.length ? STALE_DATA_SUFFIX : undefined,
  );
  const statsLoadError = useSwrLoadError(
    statsError,
    "die Übersicht der Zahlen",
    () => void mutateStats(),
  );

  const refreshAll = useCallback(async () => {
    await Promise.all([mutateOrgs(), mutateStats()]);
  }, [mutateOrgs, mutateStats]);
  // The new Träger loads after the write; a failure stands above the list,
  // also after the dialog closed (#6652).
  const createdAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Träger",
    refreshAll,
  );

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
    errorObjects: {
      softDelete: "das Löschen des Trägers",
      restore: "die Wiederherstellung des Trägers",
    },
    logEventPrefix: "organization",
    reloadObject: "die Anzeige der Träger",
  });

  const orgDeleteTargetHasSchools = useMemo(() => {
    const target = orgDelete.deleteTarget;
    if (!target) return false;
    return target.schulenCount > 0;
  }, [orgDelete.deleteTarget]);

  const {
    setDeleteTarget: setOrgDeleteTarget,
    setRestoreTarget: setOrgRestoreTarget,
  } = orgDelete;
  useLayoutEffect(() => {
    if (!organizationsAccessRefused) return;
    setCreateOrgOpen(false);
    setCreateTestOpen(false);
    setOrgDeleteTarget(null);
    setOrgRestoreTarget(null);
  }, [organizationsAccessRefused, setOrgDeleteTarget, setOrgRestoreTarget]);

  const count = organizations?.length;

  const actionButton = (
    <div className="flex items-center gap-2">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => setCreateTestOpen(true)}
      >
        Test-Träger anlegen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => setCreateOrgOpen(true)}
      >
        Neuer Träger
      </Button>
    </div>
  );

  const columns: DataTableColumn<OrganizationSummary>[] = useMemo(
    () => [
      {
        key: "name",
        header: "Träger",
        render: (row) => (
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="font-semibold text-gray-900">{row.name}</span>
              {row.testSystem ? (
                <StatusBadge label="Test-Träger" tone="orange" />
              ) : null}
            </div>
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
    <TenantPage
      title="Träger"
      stats={
        count === undefined ? undefined : (
          <TenantPageStats items={[{ value: count, label: "Träger" }]} />
        )
      }
      actions={organizationsAccessRefused ? undefined : actionButton}
      overlays={
        <>
          {orgDelete.deleteTarget && (
            <OrgSoftDeleteModal
              target={orgDelete.deleteTarget}
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

          <CreateTestTraegerModal
            isOpen={createTestOpen}
            onClose={() => setCreateTestOpen(false)}
            onCreated={async (slug) => {
              await refreshAll();
              router.push(
                `/operator/organizations/${encodeURIComponent(slug)}`,
              );
            }}
          />

          <CreateOrganizationModal
            isOpen={createOrgOpen}
            onClose={() => setCreateOrgOpen(false)}
            onCreated={createdAfterWrite.refresh}
          />
        </>
      }
    >
      <LoadErrorAlert error={statsLoadError} />
      {stats ? (
        <div className="mt-4 grid grid-cols-2 gap-3 md:grid-cols-4">
          <StatCard
            label="Träger"
            value={formatCount(stats.traegerCount)}
            tone="neutral"
          />
          <StatCard
            label="Schulen"
            value={formatCount(stats.schulenCount)}
            tone="neutral"
          />
          <StatCard
            label="Konten"
            value={formatCount(stats.kontenCount)}
            tone="neutral"
          />
          <StatCard
            label="Geräte"
            value={formatCount(stats.geraeteCount)}
            tone="neutral"
          />
        </div>
      ) : null}

      <div>
        <LoadErrorAlert error={orgsLoadError} />
        <LoadErrorAlert error={orgDelete.reloadError} />
        <LoadErrorAlert error={createdAfterWrite.error} />
        {deletedOrganizations.length > 0 && (
          <div className="mb-4 flex justify-end">
            <TrashToggleButton
              count={deletedOrganizations.length}
              active={orgDelete.showTrash}
              onToggle={() => orgDelete.setShowTrash(!orgDelete.showTrash)}
            />
          </div>
        )}

        {orgDelete.showTrash ? (
          <div className="mt-4 grid gap-4">
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
        ) : !orgsLoading &&
          !orgsError &&
          organizations !== undefined &&
          activeOrganizations.length === 0 ? (
          <ProvisioningEmptyState
            title="Keine Träger"
            description="Erstellen Sie einen neuen Träger, um Schulen zu verwalten."
            buttonLabel="Neuer Träger"
            onAction={() => setCreateOrgOpen(true)}
          />
        ) : (organizations === undefined && !orgsLoading) ||
          (orgsError && !organizations?.length) ? null : (
          <DataTable
            columns={columns}
            rows={activeOrganizations}
            getRowKey={(row) => row.id}
            onRowClick={handleRowClick}
            defaultSortKey="name"
            isLoading={orgsLoading}
          />
        )}
      </div>
    </TenantPage>
  );
}
