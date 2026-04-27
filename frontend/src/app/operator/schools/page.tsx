"use client";

import { useRouter } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { useSession } from "next-auth/react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import type { SchoolSummary } from "~/lib/operator/provisioning-helpers";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import {
  EmptyState,
  PlusIcon,
  CardSkeletons,
} from "../provisioning/provisioning-shared";
import { CreateSchoolModal } from "../provisioning/create-school-modal";
import {
  useSoftDeletable,
  DeletedEntityCard,
  SoftDeleteConfirmationModal,
  RestoreConfirmationModal,
} from "../provisioning/soft-delete-shared";

function numberFormat(value: number): string {
  return new Intl.NumberFormat("de-DE").format(value);
}

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

  const columns: DataTableColumn<SchoolSummary>[] = useMemo(
    () => [
      {
        key: "name",
        header: "Schule",
        render: (row) => (
          <div>
            <div className="flex items-center gap-2">
              <span className="font-semibold text-gray-900">{row.name}</span>
              {row.hidden && (
                <span className="rounded-full bg-[#EAB308]/15 px-2 py-0.5 text-xs font-medium text-[#854D0E]">
                  Verborgen
                </span>
              )}
            </div>
            <div className="font-mono text-xs text-gray-500">
              {row.subdomain}
            </div>
          </div>
        ),
        sortValue: (row) => row.name.toLowerCase(),
      },
      {
        key: "traeger",
        header: "Träger",
        render: (row) => (
          <span className="text-gray-700">{row.organizationName}</span>
        ),
        sortValue: (row) => row.organizationName.toLowerCase(),
      },
      {
        key: "konten",
        header: "Konten",
        align: "right",
        render: (row) => numberFormat(row.kontenCount),
        sortValue: (row) => row.kontenCount,
      },
      {
        key: "geraete",
        header: "Geräte",
        align: "right",
        render: (row) => numberFormat(row.geraeteCount),
        sortValue: (row) => row.geraeteCount,
      },
      {
        key: "personen",
        header: "Personen",
        align: "right",
        render: (row) => numberFormat(row.personenCount),
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
            <SoftDeleteConfirmationModal
              target={schoolDelete.deleteTarget}
              entityLabel="Schule"
              entityArticleAccusative="die Schule"
              nameLabel="Geben Sie den Schulnamen ein:"
              warningTitle="Folgende Aktionen werden ausgeführt:"
              warningBullets={[
                "Die Schule wird deaktiviert",
                "Alle Zugänge dieser Schule werden gesperrt",
                "Die Schule kann später wiederhergestellt werden",
              ]}
              inputId="delete-school-confirm"
              confirmInput={schoolDelete.deleteConfirmInput}
              onConfirmInputChange={schoolDelete.setDeleteConfirmInput}
              errorMessage={schoolDelete.softDeleteError}
              isProcessing={schoolDelete.isProcessing}
              onCancel={() => schoolDelete.setDeleteTarget(null)}
              onConfirm={() => void schoolDelete.handleSoftDelete()}
            />
          )}

          <RestoreConfirmationModal
            target={schoolDelete.restoreTarget}
            setTarget={schoolDelete.setRestoreTarget}
            entityLabel="Schule"
            entityArticleAccusative="die Schule"
            entityPronounNominative="Die Schule"
            entityPossessiveAccusative="ihren"
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
