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
  OrgAccount,
  OrganizationSummary,
  SchoolAccount,
  SchoolSummary,
  TraegerMember,
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
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { AccountsTable } from "~/components/operator/accounts-table";
import { DevicesTable } from "~/components/operator/devices-table";
import { DeleteDeviceModal } from "~/components/operator/delete-device-modal";
import { TransferDeviceModal } from "~/components/operator/transfer-device-modal";
import { PersonsTable } from "~/components/operator/persons-table";
import { DataTable, DataTableStatusBadge } from "~/components/ui/data-table";
import { buildSchoolColumns } from "~/components/operator/school-table-columns";
import { ConfirmationModal } from "~/components/ui/modal";
import { CaregiverCapabilityModal } from "~/components/teachers/caregiver-capability-modal";
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
import {
  PlusIcon,
  SimpleEmptyState,
} from "~/app/operator/provisioning/provisioning-shared";
import { CreateSchoolModal } from "~/app/operator/provisioning/create-school-modal";
import { EditOrganizationModal } from "~/app/operator/provisioning/edit-organization-modal";
import { MFAAdminOverrideModal } from "~/components/auth/mfa-admin-override-modal";
import { AssignTraegerMemberModal } from "~/app/operator/provisioning/assign-traeger-member-modal";
import { InviteTraegerAccountModal } from "~/components/traeger/traeger-account-modals";
import { TraegerBelegRetentionSection } from "./traeger-beleg-retention-section";
import { TraegerErasureOrderSection } from "./traeger-erasure-order-section";
import { TestTraegerSection } from "./test-traeger-section";
import { TraegerSettingsSection } from "./traeger-settings-section";
import { StatusBadge } from "~/components/ui/status-badge";
import { TraegerBueroTable } from "./traeger-buero-table";
import { CreateDeviceModal } from "~/app/operator/provisioning/create-device-modal";
import { SetApiKeyModal } from "~/app/operator/provisioning/set-api-key-modal";
import {
  DeletedEntityCard,
  TrashToggleButton,
  useSoftDeletable,
} from "~/app/operator/provisioning/soft-delete-shared";
import {
  OrgSoftDeleteModal,
  SchoolRestoreModal,
} from "~/app/operator/provisioning/operator-entity-modals";

const logger = createLogger({ component: "OperatorOrganizationDetailPage" });

const ORGANIZATIONS_BACK = {
  href: "/operator/organizations",
  label: "Zurück zu den Trägern",
} as const;

const TAB_ITEMS = [
  { id: "uebersicht", label: "Übersicht" },
  { id: "schulen", label: "Schulen" },
  { id: "konten", label: "Konten" },
  { id: "geraete", label: "Geräte" },
  { id: "personen", label: "Personen" },
  { id: "traeger-buero", label: "Träger-Büro" },
  { id: "einstellungen", label: "Einstellungen" },
  { id: "belege", label: "Belegaufbewahrung" },
  { id: "loeschauftrag", label: "Löschauftrag" },
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
  const activeTab: TabId = isTabId(tabParam) ? tabParam : "uebersicht";
  const [caregiverAccount, setCaregiverAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [caregiverSchoolContext, setCaregiverSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [editOrgOpen, setEditOrgOpen] = useState(false);
  const { show: showToggleError } = useApiErrorDisplay();
  const [createSchoolOpen, setCreateSchoolOpen] = useState(false);
  const [createDeviceOpen, setCreateDeviceOpen] = useState(false);
  const [setKeyDevice, setSetKeyDevice] = useState<OperatorDevice | null>(null);
  const [transferDevice, setTransferDevice] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteDevice, setDeleteDevice] = useState<OperatorDevice | null>(null);
  const [assignTraegerOpen, setAssignTraegerOpen] = useState(false);
  const [inviteTraegerOpen, setInviteTraegerOpen] = useState(false);
  const [mfaResetMember, setMfaResetMember] = useState<TraegerMember | null>(
    null,
  );
  const [deactivateOpen, setDeactivateOpen] = useState(false);

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
    isLoading,
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
  const organizationId = organization?.id;
  const invalidate = useInvalidateMatching();

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
    error: schoolsError,
    isLoading: schoolsLoading,
    mutate: mutateSchoolSummaries,
  } = useSWRAuth(
    isAuthenticated && organizationId
      ? operatorProvisioningKeys.organizationSchoolSummaries(organizationId)
      : null,
    async () =>
      (
        await operatorProvisioningService.listOrganizationSchools(
          organizationId ?? "",
        )
      ).filter((item) => item.organizationId === organizationId),
    {
      keepPreviousData: false,
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
    await Promise.all([
      mutateOrganizations(),
      mutateSchoolSummaries(),
      invalidate(
        (key) =>
          operatorProvisioningKeys.summaryChanges(key) &&
          key !== operatorProvisioningKeys.organizationSummaries() &&
          key !==
            operatorProvisioningKeys.organizationSchoolSummaries(
              organizationId,
            ),
      ),
    ]);
  }, [mutateOrganizations, mutateSchoolSummaries, invalidate, organizationId]);

  const accountsActive =
    activeTab === "konten" && isAuthenticated && organization != null;
  const {
    data: orgAccounts,
    error: accountsError,
    isLoading: accountsLoading,
    mutate: mutateOrgAccounts,
  } = useSWRAuth(
    accountsActive
      ? operatorProvisioningKeys.orgAccounts(organization?.id)
      : null,
    () =>
      operatorProvisioningService.listOrganizationAccounts(
        organization?.id ?? "",
      ),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const devicesActive =
    activeTab === "geraete" && isAuthenticated && organization != null;
  const {
    data: orgDevices,
    error: devicesError,
    isLoading: devicesLoading,
    mutate: mutateOrgDevices,
  } = useSWRAuth(
    devicesActive
      ? operatorProvisioningKeys.orgDevices(organization?.id)
      : null,
    () =>
      operatorProvisioningService.listOrganizationDevices(
        organization?.id ?? "",
      ),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const personsActive =
    activeTab === "personen" && isAuthenticated && organization != null;
  const {
    data: orgPersons,
    error: personsError,
    isLoading: personsLoading,
    mutate: mutateOrgPersons,
  } = useSWRAuth(
    personsActive
      ? operatorProvisioningKeys.orgPersons(organization?.id)
      : null,
    () =>
      operatorProvisioningService.listOrganizationPersons(
        organization?.id ?? "",
      ),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const traegerMembersActive =
    activeTab === "traeger-buero" && isAuthenticated && organization != null;
  const {
    data: traegerMembers,
    error: traegerMembersError,
    isLoading: traegerMembersLoading,
    mutate: mutateTraegerMembers,
  } = useSWRAuth(
    traegerMembersActive
      ? operatorProvisioningKeys.orgTraegerMembers(organization?.id)
      : null,
    () =>
      operatorProvisioningService.listTraegerMembers(organization?.id ?? ""),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );
  const {
    data: traegerInvitations,
    error: traegerInvitationsError,
    isLoading: traegerInvitationsLoading,
    mutate: mutateTraegerInvitations,
  } = useSWRAuth(
    traegerMembersActive
      ? operatorProvisioningKeys.orgTraegerInvitations(organization?.id)
      : null,
    () =>
      operatorProvisioningService.listTraegerInvitations(
        organization?.id ?? "",
      ),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );
  const reloadTraegerBuero = useCallback(
    () => Promise.all([mutateTraegerMembers(), mutateTraegerInvitations()]),
    [mutateTraegerMembers, mutateTraegerInvitations],
  );
  const traegerBueroError = traegerMembersError ?? traegerInvitationsError;
  const traegerBueroLoading =
    traegerMembersLoading || traegerInvitationsLoading;
  const traegerBueroEmpty =
    traegerMembers?.length === 0 && traegerInvitations?.length === 0;

  const organizationsLoadError = useSwrLoadError(
    organizationsError,
    "die Übersicht des Trägers",
    () => void mutateOrganizations(),
  );
  const schoolsLoadError = useSwrLoadError(
    schoolsError,
    "die Liste der Schulen",
    () => void mutateSchoolSummaries(),
    organizationSchoolSummaries?.length ? STALE_DATA_SUFFIX : undefined,
  );
  const accountsLoadError = useSwrLoadError(
    accountsError,
    "die Liste der Konten",
    () => void mutateOrgAccounts(),
    orgAccounts?.length ? STALE_DATA_SUFFIX : undefined,
  );
  const devicesLoadError = useSwrLoadError(
    devicesError,
    "die Liste der Geräte",
    () => void mutateOrgDevices(),
    orgDevices?.length ? STALE_DATA_SUFFIX : undefined,
  );
  const personsLoadError = useSwrLoadError(
    personsError,
    "die Liste der Personen",
    () => void mutateOrgPersons(),
    orgPersons?.length ? STALE_DATA_SUFFIX : undefined,
  );

  const traegerMembersLoadError = useSwrLoadError(
    traegerBueroError,
    "die Liste des Träger-Büros",
    () => void reloadTraegerBuero(),
    traegerMembers?.length || traegerInvitations?.length
      ? STALE_DATA_SUFFIX
      : undefined,
  );

  // A write's reload runs after the write; its failure is a load error above
  // the data it reloads, never an error of the closed dialog (#6652).
  const organizationAfterWrite = useRefreshAfterWrite(
    "die Anzeige des Trägers",
    refreshOrganizationDrillIn,
  );
  const editAfterWrite = useRefreshAfterWrite(
    "die Anzeige des Trägers",
    async () => {
      const refreshed = await mutateOrganizations();
      const updated = refreshed?.find((item) => item.id === organizationId);
      if (updated && updated.slug !== slug) {
        router.replace(
          `/operator/organizations/${encodeURIComponent(updated.slug)}${currentTabSearch}`,
        );
      }
    },
  );
  const statusAfterWrite = useRefreshAfterWrite(
    "die Anzeige des Trägers",
    async () => {
      await refreshOrganizationDrillIn();
      await revalidateTenantCache(activeSchools.map((item) => item.subdomain));
    },
  );
  const schoolCreatedAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Schulen",
    async (subdomain: string) => {
      await refreshOrganizationDrillIn();
      await revalidateTenantCache([subdomain]);
    },
  );
  const accountsAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Konten",
    () => mutateOrgAccounts(),
  );
  const devicesAfterWrite = useRefreshAfterWrite("die Anzeige der Geräte", () =>
    Promise.all([mutateOrgDevices(), refreshOrganizationDrillIn()]),
  );
  const bueroAfterWrite = useRefreshAfterWrite(
    "die Anzeige des Träger-Büros",
    reloadTraegerBuero,
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
    try {
      await operatorProvisioningService.updateOrganization(organization.id, {
        name: organization.name,
        slug: organization.slug,
        active: !organization.active,
      });
    } catch (error) {
      logger.warn("organization_toggle_active_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      // The retry sets the same target state, so it is safe to repeat.
      void showToggleError(error, {
        object: "die Änderung des Status",
        retry: () => void handleToggleOrgActive(),
      });
      return;
    }
    await statusAfterWrite.refresh();
  }, [organization, showToggleError, statusAfterWrite]);

  const orgDelete = useSoftDeletable<OrganizationSummary>({
    softDeleteFn: operatorProvisioningService.softDeleteOrganization,
    restoreFn: operatorProvisioningService.restoreOrganization,
    mutateList: refreshOrganizationDrillIn,
    errorObjects: {
      softDelete: "das Löschen des Trägers",
      restore: "die Wiederherstellung des Trägers",
    },
    logEventPrefix: "organization",
    reloadObject: "die Anzeige des Trägers",
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
    errorObjects: {
      softDelete: "das Löschen der Schule",
      restore: "die Wiederherstellung der Schule",
    },
    logEventPrefix: "school",
    reloadObject: "die Anzeige der Schulen",
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
        {organization?.testSystem ? (
          <StatusBadge label="Test-Träger" tone="orange" />
        ) : null}
        <button
          type="button"
          onClick={() =>
            organization?.active
              ? setDeactivateOpen(true)
              : void handleToggleOrgActive()
          }
          className="cursor-pointer"
          title={organization?.active ? "Deaktivieren" : "Aktivieren"}
          aria-label={organization?.active ? "Deaktivieren" : "Aktivieren"}
        >
          <DataTableStatusBadge active={organization?.active ?? false} />
        </button>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => setEditOrgOpen(true)}
        >
          Bearbeiten
        </Button>
        <Button
          type="button"
          variant="outline_danger"
          size="compact"
          onClick={() => {
            if (organization) {
              orgDelete.setDeleteTarget(organization);
            }
          }}
        >
          Löschen
        </Button>
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
            <TrashToggleButton
              count={deletedSchools.length}
              active={schoolDelete.showTrash}
              onToggle={() =>
                schoolDelete.setShowTrash(!schoolDelete.showTrash)
              }
            />
          )}
          <Button
            type="button"
            variant="primary"
            size="compact"
            onClick={() => setCreateSchoolOpen(true)}
          >
            <PlusIcon />
            Neue Schule
          </Button>
        </>
      );
    }

    if (activeTab === "traeger-buero") {
      return (
        <>
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => setAssignTraegerOpen(true)}
          >
            Person zuordnen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="compact"
            onClick={() => setInviteTraegerOpen(true)}
          >
            <PlusIcon />
            Person einladen
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
  }, [activeTab, deletedSchools.length, schoolDelete]);

  if (isLoading && !organization) {
    return (
      <TenantPage
        title="Träger"
        back
        backHref={ORGANIZATIONS_BACK.href}
        backLabel={ORGANIZATIONS_BACK.label}
        loading
        loadingLabel="Träger wird geladen"
      />
    );
  }

  if (!organization && organizationsLoadError) {
    return (
      <TenantPage
        title="Träger"
        back
        backHref={ORGANIZATIONS_BACK.href}
        backLabel={ORGANIZATIONS_BACK.label}
        error={organizationsLoadError}
      />
    );
  }

  if (!organization) {
    logger.warn("organization_not_found_by_slug", { slug });
    return (
      <TenantPage
        title="Träger"
        back
        backHref={ORGANIZATIONS_BACK.href}
        backLabel={ORGANIZATIONS_BACK.label}
        empty={{ title: "Träger nicht gefunden." }}
      />
    );
  }

  return (
    <TenantPage
      title={organization.name}
      back
      backHref={ORGANIZATIONS_BACK.href}
      backLabel={ORGANIZATIONS_BACK.label}
      stats={
        <EntityStatusLine
          subdomain={organization.slug}
          active={organization.active}
        />
      }
      actions={
        <>
          {orgHeaderActions}
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
        label: "Bereiche des Trägers",
      }}
      overlays={
        <>
          <ConfirmationModal
            isOpen={deactivateOpen}
            onClose={() => setDeactivateOpen(false)}
            onConfirm={() => {
              setDeactivateOpen(false);
              void handleToggleOrgActive();
            }}
            title="Träger deaktivieren?"
            confirmText="Deaktivieren"
            confirmVariant="warning"
          >
            <p className="text-sm text-gray-700">
              Alle Personen im Träger-Portal von {organization.name} werden
              sofort abgemeldet und können sich nicht mehr anmelden. Die Schulen
              des Trägers arbeiten weiter. Sie können den Träger wieder
              aktivieren.
            </p>
          </ConfirmationModal>

          <AssignTraegerMemberModal
            isOpen={assignTraegerOpen}
            onClose={() => setAssignTraegerOpen(false)}
            organizationId={organization.id}
            organizationName={organization.name}
            onAssigned={bueroAfterWrite.refresh}
          />

          <InviteTraegerAccountModal
            isOpen={inviteTraegerOpen}
            onClose={() => setInviteTraegerOpen(false)}
            onInvited={bueroAfterWrite.refresh}
            invite={(input) =>
              operatorProvisioningService.inviteTraegerMember(organization.id, {
                email: input.email,
                roles: [...input.roles],
              })
            }
            title="Person ins Träger-Büro einladen"
            hint={`Die Person bekommt eine E-Mail mit einem Link zum Träger-Portal von ${organization.name}. Dort legt sie ein Passwort fest oder meldet sich mit ihrem moto-Konto an. Ein Schulkonto braucht sie nicht.`}
            successText="Die Einladung ist verschickt. Sie steht jetzt in der Liste."
          />

          {mfaResetMember ? (
            <MFAAdminOverrideModal
              isOpen={true}
              onClose={() => setMfaResetMember(null)}
              scope="traeger"
              organizationId={organization.id}
              accountId={mfaResetMember.accountId}
              accountLabel={mfaResetMember.email}
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
            onCreated={schoolCreatedAfterWrite.refresh}
          />

          <EditOrganizationModal
            isOpen={editOrgOpen}
            onClose={() => setEditOrgOpen(false)}
            organization={summaryToOrganization(organization)}
            onUpdated={editAfterWrite.refresh}
          />

          <CreateDeviceModal
            isOpen={createDeviceOpen}
            onClose={() => setCreateDeviceOpen(false)}
            schools={organizationSchoolsForDeviceModal}
            onCreated={() => {
              void Promise.all([
                mutateOrgDevices(),
                refreshOrganizationDrillIn(),
              ]);
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

          <TransferDeviceModal
            device={transferDevice}
            schools={organizationSchoolsForDeviceModal}
            onClose={() => setTransferDevice(null)}
            onTransferred={devicesAfterWrite.refresh}
          />

          <DeleteDeviceModal
            device={deleteDevice}
            onClose={() => setDeleteDevice(null)}
            onDeleted={devicesAfterWrite.refresh}
          />
        </>
      }
    >
      <LoadErrorAlert error={organizationsLoadError} />
      <LoadErrorAlert error={orgDelete.reloadError} />
      <LoadErrorAlert error={organizationAfterWrite.error} />
      <LoadErrorAlert error={editAfterWrite.error} />
      <LoadErrorAlert error={statusAfterWrite.error} />

      {activeTab === "uebersicht" ? (
        <>
          <EntityStats stats={headerStats} createdAt={organization.createdAt} />
          {organization.testSystem ? (
            <TestTraegerSection
              organizationId={organization.id}
              organizationActive={organization.active}
              onChanged={organizationAfterWrite.refresh}
            />
          ) : null}
        </>
      ) : null}

      {activeTab === "schulen" ? (
        <>
          <LoadErrorAlert error={schoolsLoadError} className="mb-4" />
          <LoadErrorAlert error={schoolDelete.reloadError} className="mb-4" />
          <LoadErrorAlert
            error={schoolCreatedAfterWrite.error}
            className="mb-4"
          />
          {schoolDelete.showTrash ? (
            <div className="grid gap-4">
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
          ) : !schoolsLoading &&
            !schoolsError &&
            organizationSchoolSummaries !== undefined &&
            activeSchools.length === 0 ? (
            <SectionCard>
              <EmptyState
                title="Keine Schulen"
                description="Erstellen Sie eine neue Schule über die Kopfkarte."
              />
            </SectionCard>
          ) : (organizationSchoolSummaries === undefined && !schoolsLoading) ||
            (schoolsError && !organizationSchoolSummaries?.length) ? null : (
            <DataTable
              columns={schoolColumns}
              rows={activeSchools}
              getRowKey={(row) => row.id}
              onRowClick={handleSchoolClick}
              defaultSortKey="name"
              isLoading={schoolsLoading}
            />
          )}
        </>
      ) : null}

      {activeTab === "konten" ? (
        <>
          <LoadErrorAlert error={accountsLoadError} className="mb-4" />
          <LoadErrorAlert error={accountsAfterWrite.error} className="mb-4" />
          {(orgAccounts === undefined && !accountsLoading) ||
          (accountsError && !orgAccounts?.length) ? null : !accountsError &&
            !accountsLoading &&
            orgAccounts?.length === 0 ? (
            <SectionCard>
              <EmptyState title="Keine Konten für diesen Träger." />
            </SectionCard>
          ) : (
            <AccountsTable
              accounts={orgAccounts ?? []}
              showSchool
              isLoading={accountsLoading}
              onManageCaregiver={openCaregiverModal}
            />
          )}
        </>
      ) : null}

      {activeTab === "geraete" ? (
        <>
          <LoadErrorAlert error={devicesLoadError} className="mb-4" />
          <LoadErrorAlert error={devicesAfterWrite.error} className="mb-4" />
          {(orgDevices === undefined && !devicesLoading) ||
          (devicesError && !orgDevices?.length) ? null : !devicesError &&
            !devicesLoading &&
            orgDevices?.length === 0 ? (
            <SectionCard>
              <EmptyState title="Keine Geräte für diesen Träger." />
            </SectionCard>
          ) : (
            <DevicesTable
              devices={orgDevices ?? []}
              showSchool
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
          {(orgPersons === undefined && !personsLoading) ||
          (personsError && !orgPersons?.length) ? null : !personsError &&
            !personsLoading &&
            orgPersons?.length === 0 ? (
            <SectionCard>
              <EmptyState title="Keine Personen für diesen Träger." />
            </SectionCard>
          ) : (
            <PersonsTable
              persons={orgPersons ?? []}
              showSchool
              isLoading={personsLoading}
            />
          )}
        </>
      ) : null}

      {activeTab === "traeger-buero" ? (
        <>
          <LoadErrorAlert error={traegerMembersLoadError} className="mb-4" />
          <LoadErrorAlert error={bueroAfterWrite.error} className="mb-4" />
          {((traegerMembers === undefined ||
            traegerInvitations === undefined) &&
            !traegerBueroLoading) ||
          (traegerBueroError &&
            traegerBueroEmpty) ? null : !traegerBueroLoading &&
            traegerBueroEmpty ? (
            <SimpleEmptyState
              title="Noch niemand im Träger-Büro"
              description="Noch niemand hat Zugang zum Träger-Portal dieses Trägers. Laden Sie die erste Person oben mit „Person einladen“ ein."
            />
          ) : (
            <TraegerBueroTable
              organizationId={organization.id}
              members={traegerMembers ?? []}
              invitations={traegerInvitations ?? []}
              isLoading={traegerBueroLoading}
              onResetMfa={setMfaResetMember}
              onChanged={bueroAfterWrite.refresh}
            />
          )}
        </>
      ) : null}

      {activeTab === "einstellungen" ? (
        <TraegerSettingsSection organizationId={organization.id} />
      ) : null}

      {activeTab === "belege" ? (
        <TraegerBelegRetentionSection organizationId={organization.id} />
      ) : null}

      {activeTab === "loeschauftrag" ? (
        <TraegerErasureOrderSection
          organizationId={organization.id}
          organizationActive={organization.active}
        />
      ) : null}
    </TenantPage>
  );
}
