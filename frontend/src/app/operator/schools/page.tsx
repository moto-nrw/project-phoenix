"use client";

import { useRouter } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { useSession } from "next-auth/react";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import type { SchoolSummary } from "~/lib/operator/provisioning-helpers";
import { buildSchoolColumns } from "~/components/operator/school-table-columns";
import { DataTable } from "~/components/ui/data-table";
import {
  EmptyState,
  PlusIcon,
  CardSkeletons,
} from "../provisioning/provisioning-shared";
import { CreateSchoolModal } from "../provisioning/create-school-modal";
import {
  useSoftDeletable,
  DeletedEntityCard,
} from "../provisioning/soft-delete-shared";
import {
  SchoolRestoreModal,
  SchoolSoftDeleteModal,
} from "../provisioning/operator-entity-modals";

export default function OperatorSchoolsPage() {
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  useSetBreadcrumb({ pageTitle: "Schulen" });
  const router = useRouter();

  const [createSchoolOpen, setCreateSchoolOpen] = useState(false);

  const { data: organizations } = useSWR(
    isAuthenticated ? "operator-organizations" : null,
    () => operatorProvisioningService.listOrganizations(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: schoolSummaries,
    isLoading: summariesLoading,
    mutate: mutateSummaries,
  } = useSWR(
    isAuthenticated ? "operator-school-summaries" : null,
    () => operatorProvisioningService.listSchoolSummaries(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const refreshAll = useCallback(async () => {
    await mutateSummaries();
  }, [mutateSummaries]);

  const activeOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt == null) ?? [],
    [organizations],
  );

  const activeSummaries = useMemo(
    () => schoolSummaries?.filter((s) => s.deletedAt == null) ?? [],
    [schoolSummaries],
  );

  const deletedSummaries = useMemo(
    () => schoolSummaries?.filter((s) => s.deletedAt != null) ?? [],
    [schoolSummaries],
  );

  const deletedOrgIds = useMemo(
    () =>
      new Set(
        (organizations ?? [])
          .filter((o) => o.deletedAt != null)
          .map((o) => o.id),
      ),
    [organizations],
  );

  const orgSlugById = useMemo(() => {
    const map = new Map<string, string>();
    for (const org of organizations ?? []) {
      map.set(org.id, org.slug);
    }
    return map;
  }, [organizations]);

  const schoolDelete = useSoftDeletable<SchoolSummary>({
    softDeleteFn: operatorProvisioningService.softDeleteSchool,
    restoreFn: operatorProvisioningService.restoreSchool,
    mutateList: mutateSummaries,
    errorMessages: {
      softDelete:
        "Fehler beim Löschen der Schule. Bitte versuchen Sie es erneut.",
      restore:
        "Fehler beim Wiederherstellen der Schule. Bitte versuchen Sie es erneut.",
    },
    logEventPrefix: "school",
    onAfterSoftDelete: async (target) => {
      await revalidateTenantCache([target.subdomain]);
    },
    onAfterRestore: async (target) => {
      await revalidateTenantCache([target.subdomain]);
    },
  });

  const requestRestore = useCallback(
    (summary: SchoolSummary) => {
      schoolDelete.setRestoreTarget(summary);
    },
    [schoolDelete],
  );

  const schoolRestoreParentDeleted = useMemo(() => {
    const target = schoolDelete.restoreTarget;
    if (!target) return false;
    return deletedOrgIds.has(target.organizationId);
  }, [schoolDelete.restoreTarget, deletedOrgIds]);

  const tabs = useMemo(
    () => ({
      items: [
        { id: "schools", label: "Schulen", count: schoolSummaries?.length },
      ],
      activeTab: "schools",
      onTabChange: () => undefined,
    }),
    [schoolSummaries?.length],
  );

  const actionButton = (
    <button
      type="button"
      onClick={() => setCreateSchoolOpen(true)}
      className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
    >
      Neue Schule
    </button>
  );

  const mobileActionButton = (
    <button
      type="button"
      onClick={() => setCreateSchoolOpen(true)}
      className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
      aria-label="Neue Schule"
    >
      <PlusIcon />
    </button>
  );

  const handleRowClick = useCallback(
    (row: SchoolSummary) => {
      const orgSlug = orgSlugById.get(row.organizationId);
      if (!orgSlug) return;
      router.push(
        `/operator/organizations/${encodeURIComponent(orgSlug)}/schools/${encodeURIComponent(row.slug)}`,
      );
    },
    [orgSlugById, router],
  );

  const columns = useMemo(
    () => buildSchoolColumns({ showOrgColumn: true }),
    [],
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Schulen"
        tabs={tabs}
        actionButton={actionButton}
        mobileActionButton={mobileActionButton}
      />

      {summariesLoading && <CardSkeletons />}

      {!summariesLoading && (
        <div className="mt-6">
          {deletedSummaries.length > 0 && (
            <div className="mb-4 flex justify-end">
              <button
                type="button"
                onClick={() =>
                  schoolDelete.setShowTrash(!schoolDelete.showTrash)
                }
                className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  schoolDelete.showTrash
                    ? "bg-red-100 text-red-700 hover:bg-red-200"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                }`}
              >
                Papierkorb ({deletedSummaries.length})
              </button>
            </div>
          )}

          {schoolDelete.showTrash ? (
            <div className="mt-4 space-y-4">
              {deletedSummaries.map((summary) => {
                const parentOrgDeleted = deletedOrgIds.has(
                  summary.organizationId,
                );
                return (
                  <DeletedEntityCard
                    key={summary.id}
                    name={summary.name}
                    subtitle={summary.subdomain}
                    extraSubtitle={summary.organizationName}
                    deletedAt={summary.deletedAt}
                    onRestore={() => requestRestore(summary)}
                    restoreDisabled={parentOrgDeleted}
                    restoreDisabledReason={
                      parentOrgDeleted
                        ? "Träger ist gelöscht. Bitte zuerst den Träger wiederherstellen."
                        : undefined
                    }
                  />
                );
              })}
            </div>
          ) : activeSummaries.length === 0 ? (
            <EmptyState
              title="Keine Schulen"
              description="Erstellen Sie eine neue Schule unter einem Träger."
              buttonLabel="Neue Schule"
              onAction={() => setCreateSchoolOpen(true)}
            />
          ) : (
            <DataTable
              columns={columns}
              rows={activeSummaries}
              getRowKey={(row) => row.id}
              onRowClick={handleRowClick}
              defaultSortKey="name"
            />
          )}

          {schoolDelete.deleteTarget && (
            <SchoolSoftDeleteModal
              target={schoolDelete.deleteTarget}
              inputId="delete-school-confirm"
              confirmInput={schoolDelete.deleteConfirmInput}
              onConfirmInputChange={schoolDelete.setDeleteConfirmInput}
              errorMessage={schoolDelete.softDeleteError}
              isProcessing={schoolDelete.isProcessing}
              onCancel={() => schoolDelete.setDeleteTarget(null)}
              onConfirm={() => void schoolDelete.handleSoftDelete()}
            />
          )}

          <SchoolRestoreModal
            target={schoolDelete.restoreTarget}
            setTarget={schoolDelete.setRestoreTarget}
            onConfirm={() => void schoolDelete.handleRestore()}
            isProcessing={schoolDelete.isProcessing}
            errorMessage={schoolDelete.softDeleteError}
            confirmDisabled={schoolRestoreParentDeleted}
            confirmDisabledReason="Der Träger dieser Schule ist gelöscht. Bitte zuerst den Träger wiederherstellen."
          />
        </div>
      )}

      <CreateSchoolModal
        isOpen={createSchoolOpen}
        onClose={() => setCreateSchoolOpen(false)}
        organizations={activeOrganizations}
        onCreated={() => refreshAll().then(() => undefined)}
      />
    </div>
  );
}
