"use client";

import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";

import { Suspense, use, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { useSWRAuth, useInvalidateMatching } from "~/lib/swr";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import {
  operatorProvisioningService,
  revalidateTenantCache,
} from "~/lib/operator/provisioning-api";
import type {
  OperatorDevice,
  OperatorPerson,
  OrgAccount,
  OrganizationSummary,
  SchoolAccount,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import {
  summaryToOrganization,
  summaryToSchool,
} from "~/lib/operator/provisioning-helpers";
import {
  EntityStats,
  EntityStatusLine,
} from "~/components/operator/entity-head";
import { TenantPage } from "~/components/ui/tenant-page";
import { AccountsTable } from "~/components/operator/accounts-table";
import { DevicesTable } from "~/components/operator/devices-table";
import { DeleteDeviceModal } from "~/components/operator/delete-device-modal";
import { TransferDeviceModal } from "~/components/operator/transfer-device-modal";
import { DeletePersonModal } from "~/components/operator/delete-person-modal";
import { PersonsTable } from "~/components/operator/persons-table";
import { DataTableStatusBadge } from "~/components/ui/data-table";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  STALE_DATA_SUFFIX,
  useRefreshAfterWrite,
} from "~/lib/hooks/use-refresh-after-write";
import { formatCount } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";
import { PlusIcon } from "~/app/operator/provisioning/provisioning-shared";
import { EditSchoolModal } from "~/app/operator/provisioning/edit-school-modal";
import { ChildQuotaModal } from "~/app/operator/provisioning/child-quota-modal";
import { childQuotaLimit } from "~/lib/operator/child-quota";
import { InviteAdminModal } from "~/app/operator/provisioning/invite-admin-modal";
import { CreateAccountModal } from "~/app/operator/provisioning/create-account-modal";
import { CreateDeviceModal } from "~/app/operator/provisioning/create-device-modal";
import { SetApiKeyModal } from "~/app/operator/provisioning/set-api-key-modal";
import { useSoftDeletable } from "~/app/operator/provisioning/soft-delete-shared";
import { SchoolSoftDeleteModal } from "~/app/operator/provisioning/operator-entity-modals";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";

const logger = createLogger({ component: "OperatorSchoolDetailPage" });

const TAB_ITEMS = [
  { id: "uebersicht", label: "Übersicht" },
  { id: "konten", label: "Konten" },
  { id: "geraete", label: "Geräte" },
  { id: "personen", label: "Personen" },
] as const;

type TabId = (typeof TAB_ITEMS)[number]["id"];

interface PageProps {
  readonly params: Promise<{ slug: string; schoolSlug: string }>;
}

function isTabId(value: string | null | undefined): value is TabId {
  return TAB_ITEMS.some((t) => t.id === value);
}

export default function OperatorSchoolDetailPage({ params }: PageProps) {
  return (
    <Suspense fallback={null}>
      <OperatorSchoolDetailPageContent params={params} />
    </Suspense>
  );
}

function OperatorSchoolDetailPageContent({ params }: PageProps) {
  const { slug, schoolSlug } = use(params);
  const { status } = useSession();
  const invalidate = useInvalidateMatching();
  const isAuthenticated = status === "authenticated";
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const activeTab: TabId = isTabId(tabParam) ? tabParam : "uebersicht";
  const [caregiverAccount, setCaregiverAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [caregiverSchoolContext, setCaregiverSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [mfaAccount, setMfaAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [mfaSchoolContext, setMfaSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [editSchoolOpen, setEditSchoolOpen] = useState(false);
  const [childQuotaOpen, setChildQuotaOpen] = useState(false);
  const { show: showToggleError } = useApiErrorDisplay();
  const [inviteOpen, setInviteOpen] = useState(false);
  const [createAccountOpen, setCreateAccountOpen] = useState(false);
  const [createDeviceOpen, setCreateDeviceOpen] = useState(false);
  const [setKeyDevice, setSetKeyDevice] = useState<OperatorDevice | null>(null);
  const [transferDevice, setTransferDevice] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteDevice, setDeleteDevice] = useState<OperatorDevice | null>(null);
  const [deletePersonTarget, setDeletePersonTarget] =
    useState<OperatorPerson | null>(null);

  const currentTabSearch =
    activeTab === "uebersicht" ? "" : `?tab=${activeTab}`;

  const setActiveTab = useCallback(
    (next: TabId) => {
      const params = new URLSearchParams(searchParams.toString());
      if (next === "uebersicht") {
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
    error: organizationsError,
    mutate: mutateOrganizations,
  } = useSWRAuth(
    isAuthenticated ? operatorProvisioningKeys.organizationSummaries() : null,
    () => operatorProvisioningService.listOrganizationSummaries(),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const organization: OrganizationSummary | undefined = useMemo(
    () => organizations?.find((item) => item.slug === slug),
    [organizations, slug],
  );

  const activeOrganizations = useMemo(
    () =>
      (organizations ?? [])
        .filter((item) => item.deletedAt == null)
        .map(summaryToOrganization),
    [organizations],
  );

  const {
    data: schools,
    error: schoolsError,
    isLoading: schoolsLoading,
    mutate: mutateSchools,
  } = useSWRAuth(
    isAuthenticated ? operatorProvisioningKeys.schoolSummaries() : null,
    () => operatorProvisioningService.listSchoolSummaries(),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const school: SchoolSummary | undefined = useMemo(
    () =>
      schools?.find(
        (item) =>
          item.slug === schoolSlug && item.organizationId === organization?.id,
      ),
    [schoolSlug, schools, organization?.id],
  );

  useSetBreadcrumb({ pageTitle: school?.name ?? "Schule" });

  const refreshSchoolDetail = useCallback(async () => {
    await Promise.all([
      mutateOrganizations(),
      mutateSchools(),
      invalidate(
        (key) =>
          operatorProvisioningKeys.summaryChanges(key) &&
          key !== operatorProvisioningKeys.organizationSummaries() &&
          key !== operatorProvisioningKeys.schoolSummaries(),
      ),
    ]);
  }, [mutateOrganizations, mutateSchools, invalidate]);

  const accountsActive =
    activeTab === "konten" && isAuthenticated && school != null;
  const {
    data: schoolAccounts,
    error: accountsError,
    isLoading: accountsLoading,
    mutate: mutateSchoolAccounts,
  } = useSWRAuth(
    accountsActive ? operatorProvisioningKeys.schoolAccounts(school?.id) : null,
    () => operatorProvisioningService.listSchoolAccounts(school?.id ?? ""),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const devicesActive =
    activeTab === "geraete" && isAuthenticated && school != null;
  const {
    data: schoolDevices,
    error: devicesError,
    isLoading: devicesLoading,
    mutate: mutateSchoolDevices,
  } = useSWRAuth(
    devicesActive ? operatorProvisioningKeys.schoolDevices(school?.id) : null,
    () => operatorProvisioningService.listSchoolDevices(school?.id ?? ""),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const personsActive =
    activeTab === "personen" && isAuthenticated && school != null;
  const {
    data: schoolPersons,
    error: personsError,
    isLoading: personsLoading,
    mutate: mutateSchoolPersons,
  } = useSWRAuth(
    personsActive ? operatorProvisioningKeys.schoolPersons(school?.id) : null,
    () => operatorProvisioningService.listSchoolPersons(school?.id ?? ""),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // PWA standalone usage (#2189): one of the figures on the overview tab.
  const {
    data: pwaUsage,
    error: pwaUsageError,
    mutate: mutatePwaUsage,
  } = useSWRAuth(
    activeTab === "uebersicht" && isAuthenticated && school != null
      ? operatorProvisioningKeys.schoolPwaUsage(school.id)
      : null,
    () => operatorProvisioningService.getSchoolPWAUsage(school?.id ?? ""),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // One alert for the page head: the provider and school lists it is built
  // from, and the app usage it shows.
  const headLoadError = useSwrLoadError(
    organizationsError ?? schoolsError,
    "die Übersicht der Schule",
    () => void refreshSchoolDetail(),
  );
  const pwaUsageLoadError = useSwrLoadError(
    pwaUsageError,
    "die Auswertung der App-Nutzung",
    () => void mutatePwaUsage(),
  );
  const accountsLoadError = useSwrLoadError(
    accountsError,
    "die Liste der Konten",
    () => void mutateSchoolAccounts(),
    schoolAccounts === undefined ? undefined : STALE_DATA_SUFFIX,
  );
  const devicesLoadError = useSwrLoadError(
    devicesError,
    "die Liste der Geräte",
    () => void mutateSchoolDevices(),
    schoolDevices === undefined ? undefined : STALE_DATA_SUFFIX,
  );
  const personsLoadError = useSwrLoadError(
    personsError,
    "die Liste der Personen",
    () => void mutateSchoolPersons(),
    schoolPersons === undefined ? undefined : STALE_DATA_SUFFIX,
  );

  // A write's reload runs after the write; its failure is a load error above
  // the data it reloads, also after the dialog closed (#6652).
  const statusAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Schule",
    async (subdomain: string) => {
      await refreshSchoolDetail();
      await revalidateTenantCache([subdomain]);
    },
  );
  const editAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Schule",
    async (subdomains: readonly string[]) => {
      if (subdomains.length > 0) {
        await revalidateTenantCache([...subdomains]);
      }
      const [refreshedOrganizations] = await Promise.all([
        mutateOrganizations(),
        mutateSchools(),
      ]);
      const refreshedSchools =
        await operatorProvisioningService.listSchoolSummaries();
      const updatedSchool = refreshedSchools.find(
        (item) => item.id === school?.id,
      );
      if (!updatedSchool) {
        return;
      }
      const targetOrganization =
        refreshedOrganizations?.find(
          (item) => item.id === updatedSchool.organizationId,
        ) ??
        organizations?.find((item) => item.id === updatedSchool.organizationId);
      if (!targetOrganization) {
        return;
      }
      const nextPath = `/operator/organizations/${encodeURIComponent(targetOrganization.slug)}/schools/${encodeURIComponent(updatedSchool.slug)}${currentTabSearch}`;
      const currentPath = `/operator/organizations/${encodeURIComponent(slug)}/schools/${encodeURIComponent(schoolSlug)}${currentTabSearch}`;
      if (nextPath !== currentPath) {
        router.replace(nextPath);
      }
    },
  );
  const accountsAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Konten",
    () => mutateSchoolAccounts(),
  );
  const devicesAfterWrite = useRefreshAfterWrite("die Anzeige der Geräte", () =>
    Promise.all([mutateSchoolDevices(), refreshSchoolDetail()]),
  );
  const personsAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Personen",
    () => Promise.all([mutateSchoolPersons(), refreshSchoolDetail()]),
  );

  const selectedSchoolForTable = useMemo(
    () => (school ? summaryToSchool(school) : null),
    [school],
  );

  const transferSchools = useMemo(
    () =>
      (schools ?? [])
        .filter(
          (item) =>
            item.organizationId === organization?.id &&
            item.active &&
            item.deletedAt == null,
        )
        .map(summaryToSchool),
    [schools, organization?.id],
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

  const openMFAModal = useCallback(
    (
      account: SchoolAccount | OrgAccount,
      schoolContext: { id: string; name: string } | null,
    ) => {
      setMfaAccount(account);
      setMfaSchoolContext(schoolContext);
    },
    [],
  );

  const handleToggleSchoolActive = useCallback(async () => {
    if (!school) return;
    let subdomain: string;
    try {
      const fresh = await mutateSchools();
      const current = fresh?.find((item) => item.id === school.id) ?? school;

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
      subdomain = current.subdomain;
    } catch (error) {
      logger.warn("school_toggle_active_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      // No retry: the toggle reads the current state again, so a repeat
      // after an unknown outcome could switch the school back.
      void showToggleError(error, { object: "die Änderung des Status" });
      return;
    }
    await statusAfterWrite.refresh(subdomain);
  }, [mutateSchools, school, showToggleError, statusAfterWrite]);

  const schoolDelete = useSoftDeletable<SchoolSummary>({
    softDeleteFn: operatorProvisioningService.softDeleteSchool,
    restoreFn: operatorProvisioningService.restoreSchool,
    mutateList: refreshSchoolDetail,
    errorObjects: {
      softDelete: "das Löschen der Schule",
      restore: "die Wiederherstellung der Schule",
    },
    logEventPrefix: "school",
    reloadObject: "die Anzeige der Schule",
    onAfterSoftDelete: async (target) => {
      await revalidateTenantCache([target.subdomain]);
      router.push(
        `/operator/organizations/${encodeURIComponent(slug)}?tab=schulen`,
      );
    },
    onAfterRestore: async (target) => {
      await revalidateTenantCache([target.subdomain]);
    },
  });

  const handleTabValueChange = useCallback(
    (value: string) => {
      if (isTabId(value)) setActiveTab(value);
    },
    [setActiveTab],
  );

  const headerStats = useMemo(() => {
    if (!school) return [];
    // Honest wording: this counts standalone-mode usage in the window, never
    // "installed" — the browser offers no reliable install signal.
    const pwaTooltip =
      "App vom Startbildschirm aus benutzt (Standalone-Modus), letzte 30 Tage. Zählt keine Nutzung im Browser-Tab.";
    const pwaValue = (usage?: {
      standaloneUsers: number;
      eligibleUsers: number;
    }) =>
      usage
        ? `${formatCount(usage.standaloneUsers)} von ${formatCount(usage.eligibleUsers)}`
        : "–";
    // Kinderkontingent next to the Kontingentzahl (#3568), so the moto team
    // sees at a glance when a school needs another bundle.
    const quotaLimit = childQuotaLimit({
      bundles: school.childQuotaBundles,
      bundleSize: school.childQuotaBundleSize,
    });
    const overQuota =
      quotaLimit !== null && school.childQuotaCount > quotaLimit;
    return [
      { label: "Konten", value: formatCount(school.kontenCount) },
      { label: "Geräte", value: formatCount(school.geraeteCount) },
      { label: "Personen", value: formatCount(school.personenCount) },
      {
        label: "Kinderkontingent",
        value: quotaLimit === null ? "Keine Grenze" : formatCount(quotaLimit),
      },
      {
        label: "Kontingentzahl",
        value: formatCount(school.childQuotaCount),
        tone: overQuota ? ("orange" as const) : ("neutral" as const),
      },
      {
        label: "App-Nutzung Mitarbeitende",
        value: pwaValue(pwaUsage?.staff),
        title: pwaTooltip,
      },
      {
        label: "App-Nutzung Eltern",
        value: pwaValue(pwaUsage?.parent),
        title: pwaTooltip,
      },
    ];
  }, [school, pwaUsage]);

  const schoolHeaderActions = useMemo(
    () => (
      <>
        <button
          type="button"
          onClick={() => void handleToggleSchoolActive()}
          className="cursor-pointer"
          title={school?.active ? "Deaktivieren" : "Aktivieren"}
          aria-label={school?.active ? "Deaktivieren" : "Aktivieren"}
        >
          <DataTableStatusBadge active={school?.active ?? false} />
        </button>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => setEditSchoolOpen(true)}
        >
          Bearbeiten
        </Button>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => setChildQuotaOpen(true)}
        >
          Kinderkontingent
        </Button>
        <ButtonLink
          variant="outline"
          size="compact"
          href={
            school
              ? `/operator/schools/${school.id}/settings?back=${encodeURIComponent(
                  `/operator/organizations/${slug}/schools/${schoolSlug}${currentTabSearch}`,
                )}`
              : "#"
          }
        >
          Einstellungen
        </ButtonLink>
        <Button
          type="button"
          variant="outline_danger"
          size="compact"
          onClick={() => {
            if (school) {
              schoolDelete.setDeleteTarget(school);
            }
          }}
        >
          Löschen
        </Button>
      </>
    ),
    [
      currentTabSearch,
      handleToggleSchoolActive,
      school,
      schoolDelete,
      schoolSlug,
      slug,
    ],
  );

  const tabActions = useMemo(() => {
    if (activeTab === "konten") {
      return (
        <>
          <Button
            type="button"
            variant="primary"
            size="compact"
            onClick={() => setCreateAccountOpen(true)}
          >
            <PlusIcon />
            Konto erstellen
          </Button>
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => setInviteOpen(true)}
          >
            Admin einladen
          </Button>
        </>
      );
    }

    if (activeTab === "geraete") {
      return (
        <Button
          type="button"
          variant="primary"
          size="compact"
          onClick={() => setCreateDeviceOpen(true)}
        >
          <PlusIcon />
          Neues Gerät
        </Button>
      );
    }

    return null;
  }, [activeTab]);

  const organizationBack = {
    href: `/operator/organizations/${encodeURIComponent(slug)}?tab=schulen`,
    label: "Zurück zum Träger",
  };

  if (!school && headLoadError) {
    return (
      <TenantPage
        title="Schule"
        back
        backHref={organizationBack.href}
        backLabel={organizationBack.label}
        error={headLoadError}
      />
    );
  }

  if ((!organizations || schoolsLoading) && !school) {
    return (
      <TenantPage
        title="Schule"
        back
        backHref={organizationBack.href}
        backLabel={organizationBack.label}
        loading
        loadingLabel="Schule wird geladen"
      />
    );
  }

  if (!organization) {
    logger.warn("organization_not_found_by_slug", { slug });
    return (
      <TenantPage
        title="Schule"
        back
        backHref="/operator/organizations"
        backLabel="Zurück zu den Trägern"
        empty={{ title: "Träger nicht gefunden." }}
      />
    );
  }

  if (!school) {
    logger.warn("school_not_found_by_slug", { slug, schoolSlug });
    return (
      <TenantPage
        title="Schule"
        back
        backHref={organizationBack.href}
        backLabel={organizationBack.label}
        empty={{ title: "Schule nicht gefunden." }}
      />
    );
  }

  return (
    <TenantPage
      title={school.name}
      back
      backHref={organizationBack.href}
      backLabel={organizationBack.label}
      stats={
        <EntityStatusLine
          subdomain={school.subdomain}
          active={school.active}
          subtitle={
            <span className="flex flex-wrap items-center gap-2">
              <span>
                Träger:{" "}
                <span className="font-medium text-gray-800">
                  {organization.name}
                </span>
              </span>
              {school.hidden && (
                <StatusColorBadge
                  label="Verborgen"
                  color={LOCATION_COLORS.WARNING}
                  compact
                />
              )}
            </span>
          }
        />
      }
      actions={
        <>
          {schoolHeaderActions}
          {tabActions}
        </>
      }
      tabs={{
        value: activeTab,
        onChange: handleTabValueChange,
        items: TAB_ITEMS.map((tab) => ({
          value: tab.id,
          label: tab.label,
        })),
        label: "Bereiche der Schule",
      }}
      overlays={
        <>
          {mfaAccount && mfaSchoolContext ? (
            <MFAAdminOverrideModal
              isOpen={true}
              onClose={() => {
                setMfaAccount(null);
                setMfaSchoolContext(null);
              }}
              scope="operator"
              schoolId={mfaSchoolContext.id}
              accountId={mfaAccount.accountId}
              accountLabel={`${mfaAccount.firstName} ${mfaAccount.lastName}`.trim()}
            />
          ) : null}

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
              onUpdated={accountsAfterWrite.refresh}
            />
          ) : null}

          {schoolDelete.deleteTarget && (
            <SchoolSoftDeleteModal
              target={schoolDelete.deleteTarget}
              errorMessage={schoolDelete.softDeleteError}
              isProcessing={schoolDelete.isProcessing}
              onCancel={() => schoolDelete.setDeleteTarget(null)}
              onConfirm={() => void schoolDelete.handleSoftDelete()}
            />
          )}

          <ChildQuotaModal
            isOpen={childQuotaOpen}
            onClose={() => setChildQuotaOpen(false)}
            school={school}
            loadCurrentSchool={async () => {
              const fresh = await mutateSchools(undefined, {
                throwOnError: true,
              });
              const current = fresh?.find((item) => item.id === school.id);
              if (!current) {
                throw new Error("school not found after refresh");
              }
              return current;
            }}
            onUpdated={refreshSchoolDetail}
          />

          <EditSchoolModal
            isOpen={editSchoolOpen}
            onClose={() => setEditSchoolOpen(false)}
            school={summaryToSchool(school)}
            organizations={activeOrganizations}
            onUpdated={editAfterWrite.refresh}
          />

          <CreateAccountModal
            isOpen={createAccountOpen}
            onClose={() => setCreateAccountOpen(false)}
            schoolId={school.id}
            schoolName={school.name}
            onCreated={() => {
              void Promise.all([mutateSchoolAccounts(), refreshSchoolDetail()]);
            }}
          />

          <InviteAdminModal
            isOpen={inviteOpen}
            onClose={() => setInviteOpen(false)}
            schoolId={school.id}
            schoolName={school.name}
          />

          <CreateDeviceModal
            isOpen={createDeviceOpen}
            onClose={() => setCreateDeviceOpen(false)}
            schools={[summaryToSchool(school)]}
            onCreated={() => {
              void Promise.all([mutateSchoolDevices(), refreshSchoolDetail()]);
            }}
          />

          <SetApiKeyModal
            isOpen={setKeyDevice !== null}
            onClose={() => setSetKeyDevice(null)}
            device={setKeyDevice}
            onKeySet={() => {
              void mutateSchoolDevices();
            }}
          />

          <TransferDeviceModal
            device={transferDevice}
            schools={transferSchools}
            onClose={() => setTransferDevice(null)}
            onTransferred={devicesAfterWrite.refresh}
          />

          <DeleteDeviceModal
            device={deleteDevice}
            onClose={() => setDeleteDevice(null)}
            onDeleted={devicesAfterWrite.refresh}
          />

          <DeletePersonModal
            person={deletePersonTarget}
            onClose={() => setDeletePersonTarget(null)}
            onDeleted={personsAfterWrite.refresh}
          />
        </>
      }
    >
      <LoadErrorAlert error={headLoadError} />
      <LoadErrorAlert error={schoolDelete.reloadError} />
      <LoadErrorAlert error={statusAfterWrite.error} />
      <LoadErrorAlert error={editAfterWrite.error} />

      {activeTab === "uebersicht" ? (
        <>
          <LoadErrorAlert error={pwaUsageLoadError} />
          <EntityStats stats={headerStats} createdAt={school.createdAt} />
        </>
      ) : null}

      {activeTab === "konten" ? (
        <>
          <LoadErrorAlert error={accountsLoadError} className="mb-4" />
          <LoadErrorAlert error={accountsAfterWrite.error} className="mb-4" />
          {schoolAccounts === undefined &&
          !accountsLoading ? null : !accountsLoading &&
            schoolAccounts?.length === 0 ? (
            <SectionCard>
              <EmptyState title="Keine Konten für diese Schule." />
            </SectionCard>
          ) : (
            <AccountsTable
              accounts={schoolAccounts ?? []}
              isLoading={accountsLoading}
              selectedSchool={selectedSchoolForTable}
              onManageCaregiver={openCaregiverModal}
              onManageMFA={openMFAModal}
            />
          )}
        </>
      ) : null}

      {activeTab === "geraete" ? (
        <>
          <LoadErrorAlert error={devicesLoadError} className="mb-4" />
          <LoadErrorAlert error={devicesAfterWrite.error} className="mb-4" />
          {schoolDevices === undefined &&
          !devicesLoading ? null : !devicesLoading &&
            schoolDevices?.length === 0 ? (
            <SectionCard>
              <EmptyState title="Keine Geräte für diese Schule." />
            </SectionCard>
          ) : (
            <DevicesTable
              devices={schoolDevices ?? []}
              isLoading={devicesLoading}
              onSetKey={setSetKeyDevice}
              onTransfer={setTransferDevice}
              onDelete={setDeleteDevice}
            />
          )}
        </>
      ) : null}

      {activeTab === "personen" ? (
        <>
          <LoadErrorAlert error={personsLoadError} className="mb-4" />
          <LoadErrorAlert error={personsAfterWrite.error} className="mb-4" />
          {schoolPersons === undefined &&
          !personsLoading ? null : !personsLoading &&
            schoolPersons?.length === 0 ? (
            <SectionCard>
              <EmptyState title="Keine Personen für diese Schule." />
            </SectionCard>
          ) : (
            <PersonsTable
              persons={schoolPersons ?? []}
              isLoading={personsLoading}
              onDelete={setDeletePersonTarget}
            />
          )}
        </>
      ) : null}
    </TenantPage>
  );
}
