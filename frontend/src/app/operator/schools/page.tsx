"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR, { useSWRConfig } from "swr";
import { useSession } from "next-auth/react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import type {
  School,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import { operatorPath } from "~/lib/operator-url";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import {
  EmptyState,
  PlusIcon,
  CardSkeletons,
} from "../provisioning/provisioning-shared";
import { CreateSchoolModal } from "../provisioning/create-school-modal";
import { EditSchoolModal } from "../provisioning/edit-school-modal";
import { InviteAdminModal } from "../provisioning/invite-admin-modal";
import { CreateAccountModal } from "../provisioning/create-account-modal";
import {
  useSoftDeletable,
  DeletedEntityCard,
  SoftDeleteConfirmationModal,
  RestoreConfirmationModal,
} from "../provisioning/soft-delete-shared";

const logger = createLogger({ component: "OperatorSchoolsPage" });

const ACCOUNT_SWR_PREFIXES = [
  "operator-all-accounts",
  "operator-school-accounts-",
  "operator-org-accounts-",
];

function numberFormat(value: number): string {
  return new Intl.NumberFormat("de-DE").format(value);
}

// The summaries endpoint returns every field the School modals/soft-delete
// flow need, so we derive a `School` from a `SchoolSummary` rather than
// round-tripping an extra listSchools() call.
function summaryToSchool(summary: SchoolSummary): School {
  return {
    id: summary.id,
    organizationId: summary.organizationId,
    name: summary.name,
    slug: summary.slug,
    subdomain: summary.subdomain,
    address: summary.address,
    city: summary.city,
    zip: summary.zip,
    phone: summary.phone,
    email: summary.email,
    active: summary.active,
    hidden: summary.hidden,
    deletedAt: summary.deletedAt,
    createdAt: summary.createdAt,
    updatedAt: summary.updatedAt,
  };
}

export default function OperatorSchoolsPage() {
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  useSetBreadcrumb({ pageTitle: "Schulen" });
  const router = useRouter();

  const [createSchoolOpen, setCreateSchoolOpen] = useState(false);
  const [editSchoolOpen, setEditSchoolOpen] = useState(false);
  const [editSchoolTarget, setEditSchoolTarget] = useState<School | null>(null);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteSchoolId, setInviteSchoolId] = useState<string | null>(null);
  const [inviteSchoolName, setInviteSchoolName] = useState("");
  const [createAccountOpen, setCreateAccountOpen] = useState(false);
  const [createAccountSchoolId, setCreateAccountSchoolId] = useState<
    string | null
  >(null);
  const [createAccountSchoolName, setCreateAccountSchoolName] = useState("");
  const [schoolToggleError, setSchoolToggleError] = useState("");

  const { mutate: globalMutate } = useSWRConfig();

  const { data: organizations, mutate: mutateOrgs } = useSWR(
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

  const refreshAccounts = useCallback(() => {
    return globalMutate(
      (key: unknown) =>
        typeof key === "string" &&
        ACCOUNT_SWR_PREFIXES.some((p) => key.startsWith(p)),
    );
  }, [globalMutate]);

  const openEditSchool = useCallback((summary: SchoolSummary) => {
    setEditSchoolTarget(summaryToSchool(summary));
    setEditSchoolOpen(true);
  }, []);

  const openInviteAdmin = useCallback((summary: SchoolSummary) => {
    setInviteSchoolId(summary.id);
    setInviteSchoolName(summary.name);
    setInviteOpen(true);
  }, []);

  const openCreateAccount = useCallback((summary: SchoolSummary) => {
    setCreateAccountSchoolId(summary.id);
    setCreateAccountSchoolName(summary.name);
    setCreateAccountOpen(true);
  }, []);

  const openSchoolAccounts = useCallback(
    (summary: SchoolSummary) => {
      const target = operatorPath(
        `/operator/accounts?orgId=${encodeURIComponent(summary.organizationId)}&schoolId=${encodeURIComponent(summary.id)}`,
      );
      router.push(target);
    },
    [router],
  );

  const handleToggleSchoolActive = useCallback(
    async (summary: SchoolSummary) => {
      setSchoolToggleError("");
      try {
        const fresh = await mutateSummaries();
        const current = fresh?.find((s) => s.id === summary.id) ?? summary;

        await operatorProvisioningService.updateSchool(current.id, {
          organization_id: parseInt(current.organizationId, 10),
          name: current.name,
          slug: current.slug,
          subdomain: current.subdomain,
          address: current.address,
          city: current.city,
          zip: current.zip,
          phone: current.phone,
          email: current.email,
          active: !current.active,
          hidden: current.hidden,
        });
        await refreshAll();
        await revalidateTenantCache([current.subdomain]);
      } catch (error) {
        setSchoolToggleError(
          "Fehler beim Ändern des Status. Bitte versuchen Sie es erneut.",
        );
        logger.error("school_toggle_active_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [mutateSummaries, refreshAll],
  );

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

  const requestDelete = useCallback(
    (summary: SchoolSummary) => {
      schoolDelete.setDeleteTarget(summary);
    },
    [schoolDelete],
  );

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
                <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700">
                  Verborgen
                </span>
              )}
            </div>
            <div className="font-mono text-xs text-gray-500">
              {row.subdomain}
            </div>
          </div>
        ),
      },
      {
        key: "traeger",
        header: "Träger",
        render: (row) => (
          <span className="text-gray-700">{row.organizationName}</span>
        ),
      },
      {
        key: "konten",
        header: "Konten",
        align: "right",
        render: (row) => numberFormat(row.kontenCount),
      },
      {
        key: "geraete",
        header: "Geräte",
        align: "right",
        render: (row) => numberFormat(row.geraeteCount),
      },
      {
        key: "personen",
        header: "Personen",
        align: "right",
        render: (row) => numberFormat(row.personenCount),
      },
      {
        key: "status",
        header: "Status",
        render: (row) => (
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              void handleToggleSchoolActive(row);
            }}
            className="cursor-pointer"
            title={row.active ? "Deaktivieren" : "Aktivieren"}
            aria-label={row.active ? "Deaktivieren" : "Aktivieren"}
          >
            <DataTableStatusBadge active={row.active} />
          </button>
        ),
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (row) => (
          <div
            className="flex flex-wrap justify-end gap-1.5"
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
            role="presentation"
          >
            <button
              type="button"
              onClick={() => openSchoolAccounts(row)}
              className="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
            >
              Konten
            </button>
            <button
              type="button"
              onClick={() => openEditSchool(row)}
              className="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
            >
              Bearbeiten
            </button>
            <button
              type="button"
              onClick={() => openCreateAccount(row)}
              className="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
            >
              Konto erstellen
            </button>
            <button
              type="button"
              onClick={() => openInviteAdmin(row)}
              className="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
            >
              Admin einladen
            </button>
            <Link
              href={`/operator/schools/${row.id}/settings`}
              className="rounded-lg bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
            >
              Einstellungen
            </Link>
            <button
              type="button"
              onClick={() => requestDelete(row)}
              className="rounded-lg bg-red-50 px-2.5 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-100"
            >
              Löschen
            </button>
          </div>
        ),
      },
    ],
    [
      handleToggleSchoolActive,
      openEditSchool,
      openCreateAccount,
      openInviteAdmin,
      openSchoolAccounts,
      requestDelete,
    ],
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
            />
          )}

          {schoolToggleError && (
            <p className="mt-2 text-sm text-red-600">{schoolToggleError}</p>
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
      <EditSchoolModal
        isOpen={editSchoolOpen}
        onClose={() => {
          setEditSchoolOpen(false);
          setEditSchoolTarget(null);
        }}
        school={editSchoolTarget}
        organizations={activeOrganizations}
        onUpdated={async () => {
          await Promise.all([refreshAll(), mutateOrgs()]);
        }}
      />
      <InviteAdminModal
        isOpen={inviteOpen}
        onClose={() => setInviteOpen(false)}
        schoolId={inviteSchoolId}
        schoolName={inviteSchoolName}
      />
      <CreateAccountModal
        isOpen={createAccountOpen}
        onClose={() => setCreateAccountOpen(false)}
        schoolId={createAccountSchoolId}
        schoolName={createAccountSchoolName}
        onCreated={() => {
          void refreshAccounts();
        }}
      />
    </div>
  );
}
