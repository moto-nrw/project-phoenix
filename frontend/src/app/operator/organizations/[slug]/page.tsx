"use client";

import { use, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import type {
  OperatorDevice,
  OrgAccount,
  Organization,
  OrganizationSummary,
  School,
  SchoolAccount,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import { EntityHeaderCard } from "~/components/operator/entity-header-card";
import { AccountsTable } from "~/components/operator/accounts-table";
import { DevicesTable } from "~/components/operator/devices-table";
import { PersonsTable } from "~/components/operator/persons-table";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { CaregiverCapabilityModal } from "~/components/teachers";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createLogger } from "~/lib/logger";
import {
  CardSkeletons,
  EmptyState,
  PlusIcon,
} from "~/app/operator/provisioning/provisioning-shared";
import { CreateSchoolModal } from "~/app/operator/provisioning/create-school-modal";
import { EditOrganizationModal } from "~/app/operator/provisioning/edit-organization-modal";
import { CreateDeviceModal } from "~/app/operator/provisioning/create-device-modal";
import { SetApiKeyModal } from "~/app/operator/provisioning/set-api-key-modal";
import {
  DeletedEntityCard,
  RestoreConfirmationModal,
  SoftDeleteConfirmationModal,
  useSoftDeletable,
} from "~/app/operator/provisioning/soft-delete-shared";

const logger = createLogger({ component: "OperatorOrganizationDetailPage" });

function numberFormat(value: number): string {
  return new Intl.NumberFormat("de-DE").format(value);
}

function summaryToOrganization(summary: OrganizationSummary): Organization {
  return {
    id: summary.id,
    name: summary.name,
    slug: summary.slug,
    active: summary.active,
    deletedAt: summary.deletedAt,
    createdAt: summary.createdAt,
    updatedAt: summary.updatedAt,
  };
}

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

const TAB_ITEMS = [
  { id: "schulen", label: "Schulen" },
  { id: "konten", label: "Konten" },
  { id: "geraete", label: "Geräte" },
  { id: "personen", label: "Personen" },
] as const;

type TabId = (typeof TAB_ITEMS)[number]["id"];

interface PageProps {
  readonly params: Promise<{ slug: string }>;
}

function isTabId(value: string | null | undefined): value is TabId {
  return TAB_ITEMS.some((t) => t.id === value);
}

export default function OperatorOrganizationDetailPage({ params }: PageProps) {
  const { slug } = use(params);
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const activeTab: TabId = isTabId(tabParam) ? tabParam : "schulen";
  const [caregiverAccount, setCaregiverAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [caregiverSchoolContext, setCaregiverSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [editOrgOpen, setEditOrgOpen] = useState(false);
  const [orgToggleError, setOrgToggleError] = useState("");
  const [createSchoolOpen, setCreateSchoolOpen] = useState(false);
  const [createDeviceOpen, setCreateDeviceOpen] = useState(false);
  const [setKeyDevice, setSetKeyDevice] = useState<OperatorDevice | null>(null);
  const [deleteDevice, setDeleteDeviceRaw] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteConfirmed, setDeleteConfirmed] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const setDeleteDevice = useCallback((device: OperatorDevice | null) => {
    setDeleteDeviceRaw(device);
    setDeleteConfirmed(false);
    setDeleteError("");
  }, []);

  const currentTabSearch = activeTab === "schulen" ? "" : `?tab=${activeTab}`;

  const setActiveTab = useCallback(
    (next: TabId) => {
      const params = new URLSearchParams(searchParams.toString());
      if (next === "schulen") {
        params.delete("tab");
      } else {
        params.set("tab", next);
      }
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [pathname, router, searchParams],
  );

  const {
    data: organizations,
    isLoading,
    mutate: mutateOrganizations,
  } = useSWR(
    isAuthenticated ? "operator-organization-summaries" : null,
    () => operatorProvisioningService.listOrganizationSummaries(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const organization: OrganizationSummary | undefined = useMemo(
    () => organizations?.find((item) => item.slug === slug),
    [organizations, slug],
  );
  const organizationId = organization?.id;

  const activeOrganizations = useMemo(
    () =>
      (organizations ?? [])
        .filter((item) => item.deletedAt == null)
        .map(summaryToOrganization),
    [organizations],
  );

  useSetBreadcrumb({ pageTitle: organization?.name ?? "Träger" });

  const {
    data: organizationSchoolSummaries,
    isLoading: schoolsLoading,
    mutate: mutateSchoolSummaries,
  } = useSWR(
    isAuthenticated && organizationId
      ? ["operator-organization-school-summaries", organizationId]
      : null,
    () =>
      operatorProvisioningService.listOrganizationSchools(organizationId ?? ""),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const activeSchools = useMemo(
    () =>
      (organizationSchoolSummaries ?? []).filter(
        (item) => item.deletedAt == null,
      ),
    [organizationSchoolSummaries],
  );

  const deletedSchools = useMemo(
    () =>
      (organizationSchoolSummaries ?? []).filter(
        (item) => item.deletedAt != null,
      ),
    [organizationSchoolSummaries],
  );

  const refreshOrganizationDrillIn = useCallback(async () => {
    await Promise.all([mutateOrganizations(), mutateSchoolSummaries()]);
  }, [mutateOrganizations, mutateSchoolSummaries]);

  const accountsActive =
    activeTab === "konten" && isAuthenticated && organization != null;
  const {
    data: orgAccounts,
    isLoading: accountsLoading,
    mutate: mutateOrgAccounts,
  } = useSWR(
    accountsActive ? ["operator-org-accounts", organization?.id] : null,
    () =>
      operatorProvisioningService.listOrganizationAccounts(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const devicesActive =
    activeTab === "geraete" && isAuthenticated && organization != null;
  const {
    data: orgDevices,
    isLoading: devicesLoading,
    mutate: mutateOrgDevices,
  } = useSWR(
    devicesActive ? ["operator-org-devices", organization?.id] : null,
    () =>
      operatorProvisioningService.listOrganizationDevices(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const personsActive =
    activeTab === "personen" && isAuthenticated && organization != null;
  const { data: orgPersons, isLoading: personsLoading } = useSWR(
    personsActive ? ["operator-org-persons", organization?.id] : null,
    () =>
      operatorProvisioningService.listOrganizationPersons(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const handleSchoolClick = useCallback(
    (school: SchoolSummary) => {
      router.push(
        `/operator/organizations/${encodeURIComponent(slug)}/schools/${encodeURIComponent(school.slug)}`,
      );
    },
    [router, slug],
  );

  const openCaregiverModal = useCallback(
    (
      account: SchoolAccount | OrgAccount,
      schoolContext: { id: string; name: string } | null,
    ) => {
      setCaregiverAccount(account);
      setCaregiverSchoolContext(schoolContext);
    },
    [],
  );

  const handleToggleOrgActive = useCallback(async () => {
    if (!organization) return;
    setOrgToggleError("");
    try {
      await operatorProvisioningService.updateOrganization(organization.id, {
        name: organization.name,
        slug: organization.slug,
        active: !organization.active,
      });
      await refreshOrganizationDrillIn();
      await revalidateTenantCache(activeSchools.map((item) => item.subdomain));
    } catch (error) {
      setOrgToggleError(
        "Fehler beim Ändern des Status. Bitte versuchen Sie es erneut.",
      );
      logger.error("organization_toggle_active_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }, [activeSchools, organization, refreshOrganizationDrillIn]);

  const orgDelete = useSoftDeletable<OrganizationSummary>({
    softDeleteFn: operatorProvisioningService.softDeleteOrganization,
    restoreFn: operatorProvisioningService.restoreOrganization,
    mutateList: refreshOrganizationDrillIn,
    errorMessages: {
      softDelete:
        "Fehler beim Löschen des Trägers. Bitte versuchen Sie es erneut.",
      restore:
        "Fehler beim Wiederherstellen des Trägers. Bitte versuchen Sie es erneut.",
    },
    logEventPrefix: "organization",
    onAfterSoftDelete: async () => {
      await revalidateTenantCache(activeSchools.map((item) => item.subdomain));
      router.push("/operator/organizations");
    },
    onAfterRestore: async () => {
      await revalidateTenantCache(activeSchools.map((item) => item.subdomain));
    },
  });

  const orgDeleteTargetHasSchools =
    orgDelete.deleteTarget?.schulenCount != null &&
    orgDelete.deleteTarget.schulenCount > 0;

  const schoolDelete = useSoftDeletable<SchoolSummary>({
    softDeleteFn: operatorProvisioningService.softDeleteSchool,
    restoreFn: operatorProvisioningService.restoreSchool,
    mutateList: refreshOrganizationDrillIn,
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

  const requestRestoreSchool = useCallback(
    (summary: SchoolSummary) => {
      schoolDelete.setRestoreTarget(summary);
    },
    [schoolDelete],
  );

  const handleDeleteDevice = useCallback(async () => {
    if (!deleteDevice) return;
    setDeleteLoading(true);
    setDeleteError("");
    try {
      await operatorProvisioningService.deleteDevice(deleteDevice.id);
      setDeleteDevice(null);
      setDeleteConfirmed(false);
      await Promise.all([mutateOrgDevices(), refreshOrganizationDrillIn()]);
    } catch (error) {
      logger.error("device_delete_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setDeleteError(
        error instanceof Error
          ? error.message
          : "Fehler beim Löschen des Geräts",
      );
    } finally {
      setDeleteLoading(false);
    }
  }, [
    deleteDevice,
    mutateOrgDevices,
    refreshOrganizationDrillIn,
    setDeleteDevice,
  ]);

  const orgHeaderActions = useMemo(
    () => (
      <>
        <button
          type="button"
          onClick={() => void handleToggleOrgActive()}
          className="cursor-pointer"
          title={organization?.active ? "Deaktivieren" : "Aktivieren"}
          aria-label={organization?.active ? "Deaktivieren" : "Aktivieren"}
        >
          <DataTableStatusBadge active={organization?.active ?? false} />
        </button>
        <button
          type="button"
          onClick={() => setEditOrgOpen(true)}
          className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
        >
          Bearbeiten
        </button>
        <button
          type="button"
          onClick={() => {
            if (organization) {
              orgDelete.setDeleteTarget(organization);
            }
          }}
          className="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-100"
        >
          Löschen
        </button>
      </>
    ),
    [handleToggleOrgActive, orgDelete, organization],
  );

  const schoolColumns: DataTableColumn<SchoolSummary>[] = useMemo(
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
        sortValue: (row) => row.name.toLowerCase(),
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

  const organizationSchoolsForDeviceModal = useMemo(
    () => activeSchools.map(summaryToSchool),
    [activeSchools],
  );

  const tabActions = useMemo(() => {
    if (activeTab === "schulen") {
      return (
        <>
          {deletedSchools.length > 0 && (
            <button
              type="button"
              onClick={() => schoolDelete.setShowTrash(!schoolDelete.showTrash)}
              className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                schoolDelete.showTrash
                  ? "bg-red-100 text-red-700 hover:bg-red-200"
                  : "bg-gray-100 text-gray-700 hover:bg-gray-200"
              }`}
            >
              Papierkorb ({deletedSchools.length})
            </button>
          )}
          <button
            type="button"
            onClick={() => setCreateSchoolOpen(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            <PlusIcon />
            Neue Schule
          </button>
        </>
      );
    }

    if (activeTab === "geraete") {
      return (
        <button
          type="button"
          onClick={() => setCreateDeviceOpen(true)}
          className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-800"
        >
          <PlusIcon />
          Neues Gerät
        </button>
      );
    }

    return null;
  }, [activeTab, deletedSchools.length, schoolDelete]);

  if (isLoading && !organization) {
    return (
      <div className="w-full py-10 text-center text-gray-500">
        Wird geladen…
      </div>
    );
  }

  if (!organization) {
    logger.warn("organization_not_found_by_slug", { slug });
    return (
      <div className="w-full">
        <Link
          href="/operator/organizations"
          className="text-sm text-gray-600 hover:text-gray-900"
        >
          ← Zurück zur Träger-Übersicht
        </Link>
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 text-center">
          <p className="text-gray-600">Träger nicht gefunden.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full">
      <Link
        href="/operator/organizations"
        className="mb-4 inline-flex items-center gap-2 text-sm text-gray-600 transition-colors hover:text-gray-900"
      >
        <span aria-hidden>←</span>
        <span>Zurück zur Träger-Übersicht</span>
      </Link>

      <EntityHeaderCard
        title={organization.name}
        subdomain={organization.slug}
        active={organization.active}
        createdAt={organization.createdAt}
        actions={orgHeaderActions}
        stats={[
          { label: "Schulen", value: numberFormat(organization.schulenCount) },
          { label: "Konten", value: numberFormat(organization.kontenCount) },
          { label: "Geräte", value: numberFormat(organization.geraeteCount) },
          {
            label: "Personen",
            value: numberFormat(organization.personenCount),
          },
        ]}
      />

      {orgToggleError && (
        <p className="mt-3 text-sm text-red-600">{orgToggleError}</p>
      )}

      <div className="mt-6">
        <Tabs
          value={activeTab}
          onValueChange={(value) => {
            if (isTabId(value)) setActiveTab(value);
          }}
        >
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <TabsList variant="line">
              {TAB_ITEMS.map((tab) => (
                <TabsTrigger key={tab.id} value={tab.id}>
                  {tab.label}
                </TabsTrigger>
              ))}
            </TabsList>
            {tabActions ? (
              <div className="flex flex-wrap items-center justify-end gap-2">
                {tabActions}
              </div>
            ) : null}
          </div>

          <TabsPrimitive.Content value="schulen" className="mt-4">
            {schoolsLoading ? (
              <CardSkeletons />
            ) : schoolDelete.showTrash ? (
              <div className="space-y-4">
                {deletedSchools.map((school) => (
                  <DeletedEntityCard
                    key={school.id}
                    name={school.name}
                    subtitle={school.subdomain}
                    extraSubtitle={organization.name}
                    deletedAt={school.deletedAt}
                    onRestore={() => requestRestoreSchool(school)}
                    restoreDisabled={organization.deletedAt != null}
                    restoreDisabledReason={
                      organization.deletedAt != null
                        ? "Träger ist gelöscht. Bitte zuerst den Träger wiederherstellen."
                        : undefined
                    }
                  />
                ))}
              </div>
            ) : activeSchools.length === 0 ? (
              <EmptyState
                title="Keine Schulen"
                description="Erstellen Sie eine neue Schule unter diesem Träger."
                buttonLabel="Neue Schule"
                onAction={() => setCreateSchoolOpen(true)}
              />
            ) : (
              <DataTable
                columns={schoolColumns}
                rows={activeSchools}
                getRowKey={(row) => row.id}
                onRowClick={handleSchoolClick}
                defaultSortKey="name"
              />
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="konten" className="mt-4">
            {accountsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : orgAccounts && orgAccounts.length > 0 ? (
              <AccountsTable
                accounts={orgAccounts}
                showSchool
                onManageCaregiver={openCaregiverModal}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Konten für diesen Träger.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="geraete" className="mt-4">
            {devicesLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : orgDevices && orgDevices.length > 0 ? (
              <DevicesTable
                devices={orgDevices}
                showSchool
                onSetKey={setSetKeyDevice}
                onDelete={setDeleteDevice}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Geräte für diesen Träger.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="personen" className="mt-4">
            {personsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : orgPersons && orgPersons.length > 0 ? (
              <PersonsTable persons={orgPersons} showSchool />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Personen für diesen Träger.
              </div>
            )}
          </TabsPrimitive.Content>
        </Tabs>
      </div>

      {caregiverAccount && caregiverSchoolContext ? (
        <CaregiverCapabilityModal
          isOpen={true}
          onClose={() => {
            setCaregiverAccount(null);
            setCaregiverSchoolContext(null);
          }}
          scope="operator"
          accountId={caregiverAccount.accountId}
          accountLabel={`${caregiverAccount.firstName} ${caregiverAccount.lastName}`.trim()}
          schoolId={caregiverSchoolContext.id}
          schoolName={caregiverSchoolContext.name}
          onUpdated={async () => {
            await mutateOrgAccounts();
          }}
        />
      ) : null}

      {orgDelete.deleteTarget && (
        <SoftDeleteConfirmationModal
          target={orgDelete.deleteTarget}
          entityLabel="Träger"
          entityArticleAccusative="den Träger"
          nameLabel="Geben Sie den Trägernamen ein:"
          warningTitle="Hinweis:"
          warningBullets={[
            "Alle Schulen des Trägers müssen vorher gelöscht werden",
            "Der Träger kann später wiederhergestellt werden",
          ]}
          inputId="delete-org-confirm-detail"
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
        confirmDisabled={organization.deletedAt != null}
        confirmDisabledReason="Der Träger dieser Schule ist gelöscht. Bitte zuerst den Träger wiederherstellen."
      />

      <CreateSchoolModal
        isOpen={createSchoolOpen}
        onClose={() => setCreateSchoolOpen(false)}
        organizations={activeOrganizations}
        onCreated={() => refreshOrganizationDrillIn().then(() => undefined)}
      />

      <EditOrganizationModal
        isOpen={editOrgOpen}
        onClose={() => setEditOrgOpen(false)}
        organization={summaryToOrganization(organization)}
        onUpdated={async () => {
          const refreshed = await mutateOrganizations();
          const updated = refreshed?.find(
            (item) => item.id === organization.id,
          );
          if (updated && updated.slug !== slug) {
            router.replace(
              `/operator/organizations/${encodeURIComponent(updated.slug)}${currentTabSearch}`,
            );
          }
        }}
      />

      <CreateDeviceModal
        isOpen={createDeviceOpen}
        onClose={() => setCreateDeviceOpen(false)}
        schools={organizationSchoolsForDeviceModal}
        onCreated={() => {
          void Promise.all([mutateOrgDevices(), refreshOrganizationDrillIn()]);
        }}
      />

      <SetApiKeyModal
        isOpen={setKeyDevice !== null}
        onClose={() => setSetKeyDevice(null)}
        device={setKeyDevice}
        onKeySet={() => {
          void mutateOrgDevices();
        }}
      />

      {deleteDevice && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h3 className="text-lg font-semibold text-gray-900">
              Gerät löschen
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              Möchten Sie das Gerät{" "}
              <span className="font-mono font-medium">
                {deleteDevice.deviceId}
              </span>
              {deleteDevice.name && ` (${deleteDevice.name})`} von{" "}
              <span className="font-medium">{deleteDevice.schoolName}</span>{" "}
              wirklich löschen?
            </p>
            <p className="mt-2 text-sm font-medium text-red-600">
              Diese Aktion kann nicht rückgängig gemacht werden.
            </p>

            {deleteError && (
              <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
                {deleteError}
              </div>
            )}

            {!deleteConfirmed ? (
              <div className="mt-5 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setDeleteDevice(null)}
                  className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50"
                >
                  Abbrechen
                </button>
                <button
                  type="button"
                  onClick={() => setDeleteConfirmed(true)}
                  className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700"
                >
                  Ja, löschen
                </button>
              </div>
            ) : (
              <div className="mt-5 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setDeleteDevice(null)}
                  disabled={deleteLoading}
                  className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
                >
                  Abbrechen
                </button>
                <button
                  type="button"
                  onClick={() => void handleDeleteDevice()}
                  disabled={deleteLoading}
                  className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-50"
                >
                  {deleteLoading ? "Wird gelöscht..." : "Endgültig löschen"}
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
