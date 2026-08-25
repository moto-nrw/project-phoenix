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
import { EntityHeaderCard } from "~/components/operator/entity-header-card";
import { AccountsTable } from "~/components/operator/accounts-table";
import { DevicesTable } from "~/components/operator/devices-table";
import { DeleteDeviceModal } from "~/components/operator/delete-device-modal";
import { TransferDeviceModal } from "~/components/operator/transfer-device-modal";
import { DeletePersonModal } from "~/components/operator/delete-person-modal";
import { PersonsTable } from "~/components/operator/persons-table";
import { SchoolInvoicesPanel } from "~/components/operator/school-invoices-panel";
import { DataTableStatusBadge } from "~/components/ui/data-table";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { formatCount } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";
import { PlusIcon } from "~/app/operator/provisioning/provisioning-shared";
import { EditSchoolModal } from "~/app/operator/provisioning/edit-school-modal";
import { InviteAdminModal } from "~/app/operator/provisioning/invite-admin-modal";
import { CreateAccountModal } from "~/app/operator/provisioning/create-account-modal";
import { CreateDeviceModal } from "~/app/operator/provisioning/create-device-modal";
import { SetApiKeyModal } from "~/app/operator/provisioning/set-api-key-modal";
import { useSoftDeletable } from "~/app/operator/provisioning/soft-delete-shared";
import { SchoolSoftDeleteModal } from "~/app/operator/provisioning/operator-entity-modals";
import { SkeletonRegion, DetailSkeleton } from "~/components/ui/page-skeletons";

const logger = createLogger({ component: "OperatorSchoolDetailPage" });

const TAB_ITEMS = [
  { id: "konten", label: "Konten" },
  { id: "geraete", label: "Geräte" },
  { id: "personen", label: "Personen" },
  // Zahlungsplan (#1459). Heißt "Rechnungen" und nicht "Vertrag", damit der
  // Reiter nicht mit dem gleichnamigen Einstellungs-Reiter verwechselt wird,
  // in dem Tarif und Kinderzahl stehen.
  { id: "rechnungen", label: "Rechnungen" },
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
  const { status, data: session } = useSession();
  const operatorBearerToken = session?.user?.token ?? "";
  const isAuthenticated = status === "authenticated";
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const activeTab: TabId = isTabId(tabParam) ? tabParam : "konten";
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
  const [schoolToggleError, setSchoolToggleError] = useState("");
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

  const currentTabSearch = activeTab === "konten" ? "" : `?tab=${activeTab}`;

  const setActiveTab = useCallback(
    (next: TabId) => {
      const params = new URLSearchParams(searchParams.toString());
      if (next === "konten") {
        params.delete("tab");
      } else {
        params.set("tab", next);
      }
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [pathname, router, searchParams],
  );

  const { data: organizations, mutate: mutateOrganizations } = useSWR(
    isAuthenticated ? "operator-organization-summaries" : null,
    () => operatorProvisioningService.listOrganizationSummaries(),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
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
    isLoading: schoolsLoading,
    mutate: mutateSchools,
  } = useSWR(
    isAuthenticated && organization
      ? ["operator-organization-schools", organization.id]
      : null,
    () =>
      operatorProvisioningService.listOrganizationSchools(
        organization?.id ?? "",
      ),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const school: SchoolSummary | undefined = useMemo(
    () => schools?.find((item) => item.slug === schoolSlug),
    [schoolSlug, schools],
  );

  useSetBreadcrumb({ pageTitle: school?.name ?? "Schule" });

  const refreshSchoolDetail = useCallback(async () => {
    await Promise.all([mutateOrganizations(), mutateSchools()]);
  }, [mutateOrganizations, mutateSchools]);

  const accountsActive =
    activeTab === "konten" && isAuthenticated && school != null;
  const {
    data: schoolAccounts,
    isLoading: accountsLoading,
    mutate: mutateSchoolAccounts,
  } = useSWR(
    accountsActive ? ["operator-school-accounts", school?.id] : null,
    () => operatorProvisioningService.listSchoolAccounts(school?.id ?? ""),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const devicesActive =
    activeTab === "geraete" && isAuthenticated && school != null;
  const {
    data: schoolDevices,
    isLoading: devicesLoading,
    mutate: mutateSchoolDevices,
  } = useSWR(
    devicesActive ? ["operator-school-devices", school?.id] : null,
    () => operatorProvisioningService.listSchoolDevices(school?.id ?? ""),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const personsActive =
    activeTab === "personen" && isAuthenticated && school != null;
  const {
    data: schoolPersons,
    isLoading: personsLoading,
    mutate: mutateSchoolPersons,
  } = useSWR(
    personsActive ? ["operator-school-persons", school?.id] : null,
    () => operatorProvisioningService.listSchoolPersons(school?.id ?? ""),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  // PWA standalone usage (#2189): shown in the header, so it loads with the
  // page instead of a tab.
  const { data: pwaUsage } = useSWR(
    isAuthenticated && school != null
      ? ["operator-school-pwa-usage", school.id]
      : null,
    () => operatorProvisioningService.getSchoolPWAUsage(school?.id ?? ""),
    { revalidateOnFocus: false, dedupingInterval: 5000 },
  );

  const selectedSchoolForTable = useMemo(
    () => (school ? summaryToSchool(school) : null),
    [school],
  );

  const transferSchools = useMemo(
    () =>
      (schools ?? [])
        .filter((item) => item.active && item.deletedAt == null)
        .map(summaryToSchool),
    [schools],
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
    setSchoolToggleError("");
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
      await refreshSchoolDetail();
      await revalidateTenantCache([current.subdomain]);
    } catch (error) {
      setSchoolToggleError(
        "Fehler beim Ändern des Status. Bitte versuchen Sie es erneut.",
      );
      logger.error("school_toggle_active_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
    }
  }, [mutateSchools, refreshSchoolDetail, school]);

  const schoolDelete = useSoftDeletable<SchoolSummary>({
    softDeleteFn: operatorProvisioningService.softDeleteSchool,
    restoreFn: operatorProvisioningService.restoreSchool,
    mutateList: refreshSchoolDetail,
    errorMessages: {
      softDelete:
        "Fehler beim Löschen der Schule. Bitte versuchen Sie es erneut.",
      restore:
        "Fehler beim Wiederherstellen der Schule. Bitte versuchen Sie es erneut.",
    },
    logEventPrefix: "school",
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

  const handleDeviceDeleted = useCallback(async () => {
    await Promise.all([mutateSchoolDevices(), refreshSchoolDetail()]);
  }, [mutateSchoolDevices, refreshSchoolDetail]);

  const handlePersonDeleted = useCallback(async () => {
    await Promise.all([mutateSchoolPersons(), refreshSchoolDetail()]);
  }, [mutateSchoolPersons, refreshSchoolDetail]);

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
      usage ? (
        <span title={pwaTooltip}>
          {formatCount(usage.standaloneUsers)} von{" "}
          {formatCount(usage.eligibleUsers)}
        </span>
      ) : (
        <span title={pwaTooltip}>–</span>
      );
    return [
      { label: "Konten", value: formatCount(school.kontenCount) },
      { label: "Geräte", value: formatCount(school.geraeteCount) },
      { label: "Personen", value: formatCount(school.personenCount) },
      { label: "App-Nutzung Mitarbeitende", value: pwaValue(pwaUsage?.staff) },
      { label: "App-Nutzung Eltern", value: pwaValue(pwaUsage?.parent) },
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
        <button
          type="button"
          onClick={() => setEditSchoolOpen(true)}
          className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
        >
          Bearbeiten
        </button>
        <Link
          href={
            school
              ? `/operator/schools/${school.id}/settings?back=${encodeURIComponent(
                  `/operator/organizations/${slug}/schools/${schoolSlug}`,
                )}`
              : "#"
          }
          className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
        >
          Einstellungen
        </Link>
        <button
          type="button"
          onClick={() => {
            if (school) {
              schoolDelete.setDeleteTarget(school);
            }
          }}
          className="bg-moto-red/10 text-moto-red-strong hover:bg-moto-red/15 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors"
        >
          Löschen
        </button>
      </>
    ),
    [handleToggleSchoolActive, school, schoolDelete, schoolSlug, slug],
  );

  const tabActions = useMemo(() => {
    if (activeTab === "konten") {
      return (
        <>
          <button
            type="button"
            onClick={() => setCreateAccountOpen(true)}
            className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            <PlusIcon />
            Konto erstellen
          </button>
          <button
            type="button"
            onClick={() => setInviteOpen(true)}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Admin einladen
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
  }, [activeTab]);

  if ((!organizations || schoolsLoading) && !school) {
    return (
      <SkeletonRegion label="Schule wird geladen">
        <DetailSkeleton sections={1} fieldsPerSection={3} />
      </SkeletonRegion>
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

  if (!school) {
    logger.warn("school_not_found_by_slug", { slug, schoolSlug });
    return (
      <div className="w-full">
        <Link
          href={`/operator/organizations/${encodeURIComponent(slug)}`}
          className="text-sm text-gray-600 hover:text-gray-900"
        >
          ← Zurück zu {organization.name}
        </Link>
        <div className="mt-6 rounded-xl border border-gray-200 bg-white p-6 text-center">
          <p className="text-gray-600">Schule nicht gefunden.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="w-full">
      <Link
        href={`/operator/organizations/${encodeURIComponent(slug)}`}
        className="mb-4 inline-flex items-center gap-2 text-sm text-gray-600 transition-colors hover:text-gray-900"
      >
        <span aria-hidden>←</span>
        <span>Zurück zu {organization.name}</span>
      </Link>

      <EntityHeaderCard
        title={school.name}
        concept="schools"
        subdomain={school.subdomain}
        active={school.active}
        createdAt={school.createdAt}
        actions={schoolHeaderActions}
        subtitle={
          <span className="flex flex-wrap items-center gap-2">
            <span>
              Träger:{" "}
              <span className="font-medium text-gray-800">
                {organization.name}
              </span>
            </span>
            {school.hidden && (
              <span className="bg-moto-amber/15 text-moto-amber-strong rounded-full px-2 py-0.5 text-xs font-medium">
                Verborgen
              </span>
            )}
          </span>
        }
        stats={headerStats}
      />

      {schoolToggleError && (
        <p className="text-moto-red-strong mt-3 text-sm">{schoolToggleError}</p>
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

          <TabsPrimitive.Content value="konten" className="mt-4">
            {!accountsLoading &&
            (!schoolAccounts || schoolAccounts.length === 0) ? (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Konten für diese Schule.
              </div>
            ) : (
              <AccountsTable
                accounts={schoolAccounts ?? []}
                isLoading={accountsLoading}
                selectedSchool={selectedSchoolForTable}
                onManageCaregiver={openCaregiverModal}
                onManageMFA={openMFAModal}
              />
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="geraete" className="mt-4">
            {!devicesLoading &&
            (!schoolDevices || schoolDevices.length === 0) ? (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Geräte für diese Schule.
              </div>
            ) : (
              <DevicesTable
                devices={schoolDevices ?? []}
                isLoading={devicesLoading}
                onSetKey={setSetKeyDevice}
                onTransfer={setTransferDevice}
                onDelete={setDeleteDevice}
              />
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="rechnungen" className="mt-4">
            <SchoolInvoicesPanel schoolId={school.id} />
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="personen" className="mt-4">
            {!personsLoading &&
            (!schoolPersons || schoolPersons.length === 0) ? (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Personen für diese Schule.
              </div>
            ) : (
              <PersonsTable
                persons={schoolPersons ?? []}
                isLoading={personsLoading}
                onDelete={setDeletePersonTarget}
              />
            )}
          </TabsPrimitive.Content>
        </Tabs>
      </div>

      {mfaAccount && mfaSchoolContext ? (
        <MFAAdminOverrideModal
          isOpen={true}
          onClose={() => {
            setMfaAccount(null);
            setMfaSchoolContext(null);
          }}
          scope="operator"
          schoolId={mfaSchoolContext.id}
          bearerToken={operatorBearerToken}
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
          onUpdated={async () => {
            await mutateSchoolAccounts();
          }}
        />
      ) : null}

      {schoolDelete.deleteTarget && (
        <SchoolSoftDeleteModal
          target={schoolDelete.deleteTarget}
          inputId="delete-school-confirm-detail"
          confirmInput={schoolDelete.deleteConfirmInput}
          onConfirmInputChange={schoolDelete.setDeleteConfirmInput}
          errorMessage={schoolDelete.softDeleteError}
          isProcessing={schoolDelete.isProcessing}
          onCancel={() => schoolDelete.setDeleteTarget(null)}
          onConfirm={() => void schoolDelete.handleSoftDelete()}
        />
      )}

      <EditSchoolModal
        isOpen={editSchoolOpen}
        onClose={() => setEditSchoolOpen(false)}
        school={summaryToSchool(school)}
        organizations={activeOrganizations}
        onUpdated={async () => {
          const [refreshedOrganizations] = await Promise.all([
            mutateOrganizations(),
            mutateSchools(),
          ]);
          const refreshedSchools =
            await operatorProvisioningService.listSchoolSummaries();
          const updatedSchool = refreshedSchools.find(
            (item) => item.id === school.id,
          );
          if (!updatedSchool) {
            return;
          }
          const targetOrganization =
            refreshedOrganizations?.find(
              (item) => item.id === updatedSchool.organizationId,
            ) ??
            organizations?.find(
              (item) => item.id === updatedSchool.organizationId,
            );
          if (!targetOrganization) {
            return;
          }
          const nextPath = `/operator/organizations/${encodeURIComponent(targetOrganization.slug)}/schools/${encodeURIComponent(updatedSchool.slug)}${currentTabSearch}`;
          const currentPath = `/operator/organizations/${encodeURIComponent(slug)}/schools/${encodeURIComponent(schoolSlug)}${currentTabSearch}`;
          if (nextPath !== currentPath) {
            router.replace(nextPath);
          }
        }}
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
        onTransferred={handleDeviceDeleted}
      />

      <DeleteDeviceModal
        device={deleteDevice}
        onClose={() => setDeleteDevice(null)}
        onDeleted={handleDeviceDeleted}
      />

      <DeletePersonModal
        person={deletePersonTarget}
        onClose={() => setDeletePersonTarget(null)}
        onDeleted={handlePersonDeleted}
      />
    </div>
  );
}
