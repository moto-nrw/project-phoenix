"use client";

import { Suspense, use, useCallback, useMemo, useState } from "react";
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
  OrganizationSummary,
  SchoolAccount,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import {
  summaryToOrganization,
  summaryToSchool,
} from "~/lib/operator/provisioning-helpers";
import { EntityHeaderCard } from "~/components/operator/entity-header-card";
import { AccountsTable } from "~/components/operator/accounts-table";
import { DevicesTable } from "~/components/operator/devices-table";
import { DeleteDeviceModal } from "~/components/operator/delete-device-modal";
import { PersonsTable } from "~/components/operator/persons-table";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import { buildSchoolColumns } from "~/components/operator/school-table-columns";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { formatCount } from "~/lib/format-utils";
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
  useSoftDeletable,
} from "~/app/operator/provisioning/soft-delete-shared";
import {
  OrgSoftDeleteModal,
  SchoolRestoreModal,
} from "~/app/operator/provisioning/operator-entity-modals";

const logger = createLogger({ component: "OperatorOrganizationDetailPage" });

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
  return (
    <Suspense fallback={null}>
      <OperatorOrganizationDetailPageContent params={params} />
    </Suspense>
  );
}

function OperatorOrganizationDetailPageContent({ params }: PageProps) {
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
  const [deleteDevice, setDeleteDevice] = useState<OperatorDevice | null>(null);

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

  const handleDeviceDeleted = useCallback(async () => {
    await Promise.all([mutateOrgDevices(), refreshOrganizationDrillIn()]);
  }, [mutateOrgDevices, refreshOrganizationDrillIn]);

  const handleTabValueChange = useCallback(
    (value: string) => {
      if (isTabId(value)) setActiveTab(value);
    },
    [setActiveTab],
  );

  const headerStats = useMemo(
    () =>
      organization
        ? [
            {
              label: "Schulen",
              value: formatCount(organization.schulenCount),
            },
            { label: "Konten", value: formatCount(organization.kontenCount) },
            { label: "Geräte", value: formatCount(organization.geraeteCount) },
            {
              label: "Personen",
              value: formatCount(organization.personenCount),
            },
          ]
        : [],
    [organization],
  );

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
          className="rounded-lg bg-[#FF3130]/10 px-3 py-1.5 text-xs font-medium text-[#CC2626] transition-colors hover:bg-[#FF3130]/15"
        >
          Löschen
        </button>
      </>
    ),
    [handleToggleOrgActive, orgDelete, organization],
  );

  const schoolColumns = useMemo(() => buildSchoolColumns(), []);

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
                  ? "bg-[#FF3130]/15 text-[#CC2626] hover:bg-[#FF3130]/20"
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
        stats={headerStats}
      />

      {orgToggleError && (
        <p className="mt-3 text-sm text-[#CC2626]">{orgToggleError}</p>
      )}

      <div className="mt-6">
        <Tabs value={activeTab} onValueChange={handleTabValueChange}>
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
        <OrgSoftDeleteModal
          target={orgDelete.deleteTarget}
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

      <SchoolRestoreModal
        target={schoolDelete.restoreTarget}
        setTarget={schoolDelete.setRestoreTarget}
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

      <DeleteDeviceModal
        device={deleteDevice}
        onClose={() => setDeleteDevice(null)}
        onDeleted={handleDeviceDeleted}
      />
    </div>
  );
}
