"use client";

import { useState, useMemo, useCallback } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR, { useSWRConfig } from "swr";
import { useSession } from "next-auth/react";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  Organization,
  School,
  SchoolAccount,
  OrgAccount,
  OperatorDevice,
  OperatorPerson,
} from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import {
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  formatLastSeen,
} from "~/lib/iot-helpers";
import { createLogger } from "~/lib/logger";
import {
  StatusBadge,
  EmptyState,
  SimpleEmptyState,
  PlusIcon,
  CardSkeletons,
} from "./provisioning-shared";
import { CreateOrganizationModal } from "./create-organization-modal";
import { EditOrganizationModal } from "./edit-organization-modal";
import { CreateSchoolModal } from "./create-school-modal";
import { EditSchoolModal } from "./edit-school-modal";
import { InviteAdminModal } from "./invite-admin-modal";
import { CreateAccountModal } from "./create-account-modal";
import { CreateDeviceModal } from "./create-device-modal";
import { SetApiKeyModal } from "./set-api-key-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { CaregiverCapabilityModal } from "~/components/teachers";

const logger = createLogger({ component: "OperatorProvisioningPage" });

type ActiveTab =
  | "organizations"
  | "schools"
  | "accounts"
  | "devices"
  | "persons";

export default function OperatorProvisioningPage() {
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  useSetBreadcrumb({ pageTitle: "Schulverwaltung" });

  const [activeTab, setActiveTab] = useState<ActiveTab>("organizations");
  const [createOrgOpen, setCreateOrgOpen] = useState(false);
  const [createSchoolOpen, setCreateSchoolOpen] = useState(false);
  const [editOrgOpen, setEditOrgOpen] = useState(false);
  const [editOrgTarget, setEditOrgTarget] = useState<Organization | null>(null);
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
  const [caregiverAccount, setCaregiverAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [caregiverSchoolContext, setCaregiverSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [selectedSchool, setSelectedSchool] = useState<School | null>(null);
  const [filterOrgId, setFilterOrgId] = useState<string>("");
  const [createDeviceOpen, setCreateDeviceOpen] = useState(false);
  const [setKeyDevice, setSetKeyDevice] = useState<OperatorDevice | null>(null);
  const [deleteDevice, setDeleteDeviceRaw] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteConfirmed, setDeleteConfirmed] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  // Reset confirmation state when opening/closing the delete dialog
  const setDeleteDevice = useCallback((device: OperatorDevice | null) => {
    setDeleteDeviceRaw(device);
    setDeleteConfirmed(false);
    setDeleteError("");
  }, []);

  // Person soft-delete state (triple-confirm: type name to confirm)
  const [deletePersonTarget, setDeletePersonTargetRaw] =
    useState<OperatorPerson | null>(null);
  const [deletePersonConfirmInput, setDeletePersonConfirmInput] = useState("");
  const [deletePersonLoading, setDeletePersonLoading] = useState(false);
  const [deletePersonError, setDeletePersonError] = useState("");

  const setDeletePersonTarget = useCallback((person: OperatorPerson | null) => {
    setDeletePersonTargetRaw(person);
    setDeletePersonConfirmInput("");
    setDeletePersonError("");
  }, []);

  const { mutate: globalMutate } = useSWRConfig();

  // Soft-delete / restore state (triple-confirm: type name to confirm)
  const [deleteTarget, setDeleteTargetRaw] = useState<School | null>(null);
  const [deleteConfirmInput, setDeleteConfirmInput] = useState("");
  const [restoreTarget, setRestoreTarget] = useState<School | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [showTrash, setShowTrash] = useState(false);
  const [softDeleteError, setSoftDeleteError] = useState("");

  const setDeleteTarget = useCallback((school: School | null) => {
    setDeleteTargetRaw(school);
    setDeleteConfirmInput("");
    setSoftDeleteError("");
  }, []);

  // Toggle error state
  const [orgToggleError, setOrgToggleError] = useState("");
  const [schoolToggleError, setSchoolToggleError] = useState("");

  const {
    data: organizations,
    isLoading: orgsLoading,
    mutate: mutateOrgs,
  } = useSWR(
    isAuthenticated ? "operator-organizations" : null,
    () => operatorProvisioningService.listOrganizations(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: schools,
    isLoading: schoolsLoading,
    mutate: mutateSchools,
  } = useSWR(
    isAuthenticated ? "operator-schools" : null,
    () => operatorProvisioningService.listSchools(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // School-level accounts (when a specific school is selected)
  const { data: schoolAccounts, isLoading: schoolAccountsLoading } = useSWR(
    isAuthenticated && activeTab === "accounts" && selectedSchool
      ? `operator-school-accounts-${selectedSchool.id}`
      : null,
    () => operatorProvisioningService.listSchoolAccounts(selectedSchool!.id),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // Org-level accounts (when only an org is selected, no specific school)
  const { data: orgAccounts, isLoading: orgAccountsLoading } = useSWR(
    isAuthenticated &&
      activeTab === "accounts" &&
      filterOrgId &&
      !selectedSchool
      ? `operator-org-accounts-${filterOrgId}`
      : null,
    () => operatorProvisioningService.listOrganizationAccounts(filterOrgId),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // All accounts (no filter selected)
  const { data: allAccounts, isLoading: allAccountsLoading } = useSWR(
    isAuthenticated &&
      activeTab === "accounts" &&
      !filterOrgId &&
      !selectedSchool
      ? "operator-all-accounts"
      : null,
    () => operatorProvisioningService.listAllAccounts(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // School-level devices (when a specific school is selected on devices tab)
  const { data: schoolDevices, isLoading: schoolDevicesLoading } = useSWR(
    isAuthenticated && activeTab === "devices" && selectedSchool
      ? `operator-school-devices-${selectedSchool.id}`
      : null,
    () => operatorProvisioningService.listSchoolDevices(selectedSchool!.id),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // Org-level devices (when only an org is selected, no specific school)
  const { data: orgDevices, isLoading: orgDevicesLoading } = useSWR(
    isAuthenticated && activeTab === "devices" && filterOrgId && !selectedSchool
      ? `operator-org-devices-${filterOrgId}`
      : null,
    () => operatorProvisioningService.listOrganizationDevices(filterOrgId),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // All devices (no filter selected)
  const { data: allDevices, isLoading: allDevicesLoading } = useSWR(
    isAuthenticated &&
      activeTab === "devices" &&
      !filterOrgId &&
      !selectedSchool
      ? "operator-all-devices"
      : null,
    () => operatorProvisioningService.listAllDevices(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  // School-level persons (requires a specific school selected)
  const {
    data: schoolPersons,
    isLoading: schoolPersonsLoading,
    mutate: mutateSchoolPersons,
  } = useSWR(
    isAuthenticated && activeTab === "persons" && selectedSchool
      ? `operator-school-persons-${selectedSchool.id}`
      : null,
    () => operatorProvisioningService.listSchoolPersons(selectedSchool!.id),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const accountsLoading = selectedSchool
    ? schoolAccountsLoading
    : filterOrgId
      ? orgAccountsLoading
      : allAccountsLoading;

  const devicesLoading = selectedSchool
    ? schoolDevicesLoading
    : filterOrgId
      ? orgDevicesLoading
      : allDevicesLoading;

  const DEVICE_SWR_PREFIXES = useMemo(
    () => [
      "operator-all-devices",
      "operator-school-devices-",
      "operator-org-devices-",
    ],
    [],
  );

  const refreshDevices = useCallback(() => {
    return globalMutate(
      (key: unknown) =>
        typeof key === "string" &&
        DEVICE_SWR_PREFIXES.some((p) => key.startsWith(p)),
    );
  }, [globalMutate, DEVICE_SWR_PREFIXES]);

  const ACCOUNT_SWR_PREFIXES = useMemo(
    () => [
      "operator-all-accounts",
      "operator-school-accounts-",
      "operator-org-accounts-",
    ],
    [],
  );

  const refreshAccounts = useCallback(() => {
    return globalMutate(
      (key: unknown) =>
        typeof key === "string" &&
        ACCOUNT_SWR_PREFIXES.some((p) => key.startsWith(p)),
    );
  }, [globalMutate, ACCOUNT_SWR_PREFIXES]);

  const handleDeleteDevice = useCallback(async () => {
    if (!deleteDevice) return;
    setDeleteLoading(true);
    setDeleteError("");
    try {
      await operatorProvisioningService.deleteDevice(deleteDevice.id);
      setDeleteDevice(null);
      setDeleteConfirmed(false);
      void refreshDevices();
    } catch (err) {
      logger.error("device_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setDeleteError(
        err instanceof Error ? err.message : "Fehler beim Löschen des Geräts",
      );
    } finally {
      setDeleteLoading(false);
    }
  }, [deleteDevice, refreshDevices, setDeleteDevice]);

  const handleDeletePerson = useCallback(async () => {
    if (!deletePersonTarget) return;
    setDeletePersonLoading(true);
    setDeletePersonError("");
    try {
      await operatorProvisioningService.softDeletePerson(deletePersonTarget.id);
      setDeletePersonTarget(null);
      void mutateSchoolPersons();
    } catch (err) {
      logger.error("person_soft_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setDeletePersonError(
        err instanceof Error ? err.message : "Fehler beim Löschen der Person",
      );
    } finally {
      setDeletePersonLoading(false);
    }
  }, [deletePersonTarget, mutateSchoolPersons, setDeletePersonTarget]);

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

  // Schools filtered by selected organization (for accounts tab filter)
  const filteredSchools = useMemo(() => {
    if (!schools) return [];
    if (!filterOrgId) return schools;
    return schools.filter((s) => s.organizationId === filterOrgId);
  }, [schools, filterOrgId]);

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "organizations",
          label: "Träger",
          count: organizations?.length,
        },
        { id: "schools", label: "Schulen", count: schools?.length },
        {
          id: "accounts",
          label: "Konten",
          count:
            activeTab === "accounts"
              ? selectedSchool
                ? schoolAccounts?.length
                : filterOrgId
                  ? orgAccounts?.length
                  : allAccounts?.length
              : undefined,
        },
        {
          id: "devices",
          label: "Geräte",
          count:
            activeTab === "devices"
              ? selectedSchool
                ? schoolDevices?.length
                : filterOrgId
                  ? orgDevices?.length
                  : allDevices?.length
              : undefined,
        },
        {
          id: "persons",
          label: "Personen",
          count:
            activeTab === "persons" && selectedSchool
              ? schoolPersons?.length
              : undefined,
        },
      ],
      activeTab,
      onTabChange: (tabId: string) => {
        setActiveTab(tabId as ActiveTab);
      },
    }),
    [
      activeTab,
      organizations?.length,
      schools?.length,
      selectedSchool,
      filterOrgId,
      schoolAccounts?.length,
      orgAccounts?.length,
      allAccounts?.length,
      schoolDevices?.length,
      orgDevices?.length,
      allDevices?.length,
      schoolPersons?.length,
    ],
  );

  // Open handlers
  const openEditOrg = useCallback((org: Organization) => {
    setEditOrgTarget(org);
    setEditOrgOpen(true);
  }, []);

  const openEditSchool = useCallback((school: School) => {
    setEditSchoolTarget(school);
    setEditSchoolOpen(true);
  }, []);

  const openInviteAdmin = useCallback((school: School) => {
    setInviteSchoolId(school.id);
    setInviteSchoolName(school.name);
    setInviteOpen(true);
  }, []);

  const openCreateAccount = useCallback((school: School) => {
    setCreateAccountSchoolId(school.id);
    setCreateAccountSchoolName(school.name);
    setCreateAccountOpen(true);
  }, []);

  const handleOrgFilterChange = useCallback(
    (orgId: string) => {
      setFilterOrgId(orgId);
      if (selectedSchool && orgId && selectedSchool.organizationId !== orgId) {
        setSelectedSchool(null);
      }
    },
    [selectedSchool],
  );

  const handleSchoolFilterChange = useCallback(
    (schoolId: string) => {
      if (!schoolId) {
        setSelectedSchool(null);
        return;
      }
      const school = schools?.find((s) => s.id === schoolId);
      if (school) {
        setSelectedSchool(school);
      }
    },
    [schools],
  );

  const openSchoolAccounts = useCallback((school: School) => {
    setFilterOrgId(school.organizationId);
    setSelectedSchool(school);
    setActiveTab("accounts");
  }, []);

  // Toggle handlers
  const handleToggleOrgActive = useCallback(
    async (org: Organization) => {
      setOrgToggleError("");
      try {
        await operatorProvisioningService.updateOrganization(org.id, {
          name: org.name,
          slug: org.slug,
          active: !org.active,
        });
        await mutateOrgs();
        const orgSchoolSlugs = (schools ?? [])
          .filter((s) => s.organizationId === org.id)
          .map((s) => s.subdomain);
        if (orgSchoolSlugs.length > 0) {
          try {
            await fetch("/api/operator/provisioning/revalidate-tenant", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ slugs: orgSchoolSlugs }),
            });
          } catch {
            /* Cache self-heals in ≤5 min */
          }
        }
      } catch (error) {
        setOrgToggleError(
          "Fehler beim Ändern des Status. Bitte versuchen Sie es erneut.",
        );
        logger.error("organization_toggle_active_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [mutateOrgs, schools],
  );

  const handleToggleSchoolActive = useCallback(
    async (school: School) => {
      setSchoolToggleError("");
      try {
        // Refetch to get the latest state — prevents silently reverting fields
        // (e.g. hidden) changed by another operator between page load and click.
        const freshSchools = await mutateSchools();
        const fresh = freshSchools?.find((s) => s.id === school.id);
        if (!fresh) return;

        await operatorProvisioningService.updateSchool(fresh.id, {
          organization_id: parseInt(fresh.organizationId, 10),
          name: fresh.name,
          slug: fresh.slug,
          subdomain: fresh.subdomain,
          address: fresh.address ?? "",
          city: fresh.city ?? "",
          zip: fresh.zip ?? "",
          phone: fresh.phone ?? "",
          email: fresh.email ?? "",
          active: !fresh.active,
          hidden: fresh.hidden,
        });
        await mutateSchools();
        try {
          await fetch("/api/operator/provisioning/revalidate-tenant", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ slugs: [fresh.subdomain] }),
          });
        } catch {
          /* Cache self-heals in ≤5 min */
        }
      } catch (error) {
        setSchoolToggleError(
          "Fehler beim Ändern des Status. Bitte versuchen Sie es erneut.",
        );
        logger.error("school_toggle_active_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [mutateSchools],
  );

  const activeSchools = useMemo(
    () => schools?.filter((s) => s.deletedAt == null) ?? [],
    [schools],
  );

  const deletedSchools = useMemo(
    () => schools?.filter((s) => s.deletedAt != null) ?? [],
    [schools],
  );

  const executeSchoolAction = useCallback(
    async (
      target: School | null,
      action: (id: string) => Promise<unknown>,
      errorMsg: string,
      logEvent: string,
      onSuccess: () => void,
    ) => {
      if (!target) return;
      setIsProcessing(true);
      setSoftDeleteError("");
      try {
        await action(target.id);
        // Backend action succeeded — invalidate tenant cache immediately
        // so stale routing data doesn't persist for up to 5 minutes.
        try {
          await fetch("/api/operator/provisioning/revalidate-tenant", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ slugs: [target.subdomain] }),
          });
        } catch {
          /* Cache self-heals in ≤5 min */
        }
        onSuccess();
        // SWR list refresh is best-effort — failure is cosmetic (stale list until next load)
        await mutateSchools().catch(() => {});
      } catch (error) {
        setSoftDeleteError(errorMsg);
        logger.error(logEvent, {
          error: error instanceof Error ? error.message : String(error),
        });
      } finally {
        setIsProcessing(false);
      }
    },
    [mutateSchools],
  );

  const handleSoftDelete = useCallback(
    () =>
      executeSchoolAction(
        deleteTarget,
        operatorProvisioningService.softDeleteSchool,
        "Fehler beim Löschen der Schule. Bitte versuchen Sie es erneut.",
        "school_soft_delete_failed",
        () => {
          setDeleteTarget(null);
          if (selectedSchool?.id === deleteTarget?.id) {
            setSelectedSchool(null);
          }
        },
      ),
    [deleteTarget, executeSchoolAction, selectedSchool, setDeleteTarget],
  );

  const handleRestore = useCallback(
    () =>
      executeSchoolAction(
        restoreTarget,
        operatorProvisioningService.restoreSchool,
        "Fehler beim Wiederherstellen der Schule. Bitte versuchen Sie es erneut.",
        "school_restore_failed",
        () => {
          setRestoreTarget(null);
          setShowTrash(false);
        },
      ),
    [restoreTarget, executeSchoolAction],
  );

  const isLoading =
    activeTab === "organizations"
      ? orgsLoading
      : activeTab === "schools"
        ? schoolsLoading
        : false; // accounts tab handles its own loading inline

  const actionButton =
    activeTab === "organizations" ? (
      <button
        type="button"
        onClick={() => setCreateOrgOpen(true)}
        className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        Neuer Träger
      </button>
    ) : activeTab === "schools" ? (
      <button
        type="button"
        onClick={() => setCreateSchoolOpen(true)}
        className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        Neue Schule
      </button>
    ) : null;

  const mobileActionButton =
    activeTab === "organizations" ? (
      <button
        type="button"
        onClick={() => setCreateOrgOpen(true)}
        className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
        aria-label="Neuer Träger"
      >
        <PlusIcon />
      </button>
    ) : activeTab === "schools" ? (
      <button
        type="button"
        onClick={() => setCreateSchoolOpen(true)}
        className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
        aria-label="Neue Schule"
      >
        <PlusIcon />
      </button>
    ) : null;

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Schulverwaltung"
        tabs={tabs}
        actionButton={actionButton}
        mobileActionButton={mobileActionButton}
      />

      {isLoading && <CardSkeletons />}

      {/* Organizations Tab */}
      {activeTab === "organizations" && !orgsLoading && (
        <>
          {organizations?.length === 0 && (
            <EmptyState
              title="Keine Träger"
              description="Erstellen Sie einen neuen Träger, um Schulen zu verwalten."
              buttonLabel="Neuer Träger"
              onAction={() => setCreateOrgOpen(true)}
            />
          )}
          {organizations && organizations.length > 0 && (
            <div className="mt-4 space-y-4">
              {organizations.map((org) => (
                <OrganizationCard
                  key={org.id}
                  organization={org}
                  onEdit={openEditOrg}
                  onToggleActive={handleToggleOrgActive}
                />
              ))}
            </div>
          )}
          {orgToggleError && (
            <p className="mt-2 text-sm text-red-600">{orgToggleError}</p>
          )}
        </>
      )}

      {/* Schools Tab */}
      {activeTab === "schools" && !schoolsLoading && (
        <>
          {deletedSchools.length > 0 && (
            <div className="mb-4 flex justify-end">
              <button
                type="button"
                onClick={() => setShowTrash(!showTrash)}
                className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  showTrash
                    ? "bg-red-100 text-red-700 hover:bg-red-200"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                }`}
              >
                Papierkorb ({deletedSchools.length})
              </button>
            </div>
          )}

          {showTrash ? (
            <div className="mt-4 space-y-4">
              {deletedSchools.map((school) => (
                <DeletedSchoolCard
                  key={school.id}
                  school={school}
                  onRestore={setRestoreTarget}
                />
              ))}
            </div>
          ) : (
            <>
              {activeSchools.length === 0 && (
                <EmptyState
                  title="Keine Schulen"
                  description="Erstellen Sie eine neue Schule unter einem Träger."
                  buttonLabel="Neue Schule"
                  onAction={() => setCreateSchoolOpen(true)}
                />
              )}
              {activeSchools.length > 0 && (
                <div className="mt-4 space-y-4">
                  {activeSchools.map((school) => (
                    <SchoolCard
                      key={school.id}
                      school={school}
                      onEdit={openEditSchool}
                      onToggleActive={handleToggleSchoolActive}
                      onInviteAdmin={openInviteAdmin}
                      onCreateAccount={openCreateAccount}
                      onViewAccounts={openSchoolAccounts}
                      onDelete={setDeleteTarget}
                    />
                  ))}
                </div>
              )}
            </>
          )}
          {schoolToggleError && (
            <p className="mt-2 text-sm text-red-600">{schoolToggleError}</p>
          )}

          {/* School soft-delete triple-confirm dialog */}
          {deleteTarget && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
              <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
                <h3 className="text-lg font-semibold text-gray-900">
                  Schule löschen
                </h3>
                <p className="mt-2 text-sm text-gray-600">
                  Möchten Sie die Schule{" "}
                  <span className="font-medium">{deleteTarget.name}</span>{" "}
                  wirklich löschen?
                </p>
                <div className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
                  <p className="font-medium">
                    Folgende Aktionen werden ausgeführt:
                  </p>
                  <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
                    <li>Die Schule wird deaktiviert</li>
                    <li>Alle Zugänge dieser Schule werden gesperrt</li>
                    <li>Die Schule kann später wiederhergestellt werden</li>
                  </ul>
                </div>

                <div className="mt-4">
                  <label
                    htmlFor="delete-school-confirm"
                    className="block text-sm font-medium text-gray-700"
                  >
                    Geben Sie den Schulnamen ein:
                  </label>
                  <p className="mb-1 text-sm font-medium text-gray-900">
                    {deleteTarget.name}
                  </p>
                  <input
                    id="delete-school-confirm"
                    type="text"
                    value={deleteConfirmInput}
                    onChange={(e) => setDeleteConfirmInput(e.target.value)}
                    placeholder={deleteTarget.name}
                    className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-red-500 focus:ring-1 focus:ring-red-500 focus:outline-none"
                    autoComplete="off"
                  />
                </div>

                {softDeleteError && (
                  <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
                    {softDeleteError}
                  </div>
                )}

                <div className="mt-5 flex justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => setDeleteTarget(null)}
                    disabled={isProcessing}
                    className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
                  >
                    Abbrechen
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleSoftDelete()}
                    disabled={
                      isProcessing || deleteConfirmInput !== deleteTarget.name
                    }
                    className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {isProcessing ? "Wird gelöscht..." : "Löschen"}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Restore confirmation modal */}
          <ConfirmationModal
            isOpen={restoreTarget !== null}
            onClose={() => setRestoreTarget(null)}
            onConfirm={() => void handleRestore()}
            title="Schule wiederherstellen"
            confirmText="Wiederherstellen"
            isConfirmLoading={isProcessing}
          >
            <p className="text-sm text-gray-600">
              Möchten Sie die Schule &quot;{restoreTarget?.name}&quot;
              wiederherstellen? Die Schule wird in ihren vorherigen Zustand
              zurückversetzt.
            </p>
          </ConfirmationModal>
        </>
      )}

      {/* Accounts Tab */}
      {activeTab === "accounts" && (
        <>
          <OrgSchoolFilter
            idPrefix="account"
            organizations={organizations}
            filteredSchools={filteredSchools}
            filterOrgId={filterOrgId}
            selectedSchool={selectedSchool}
            onOrgChange={handleOrgFilterChange}
            onSchoolChange={handleSchoolFilterChange}
          />

          {/* Loading */}
          {accountsLoading && <CardSkeletons />}

          {/* Org-level view (no specific school selected) */}
          {!selectedSchool && filterOrgId && !orgAccountsLoading && (
            <>
              {orgAccounts?.length === 0 && (
                <SimpleEmptyState
                  title="Keine Konten"
                  description="Für diesen Träger gibt es noch keine zugewiesenen Konten."
                />
              )}
              {orgAccounts && orgAccounts.length > 0 && (
                <AccountsTable
                  accounts={orgAccounts}
                  showSchool
                  onManageCaregiver={openCaregiverModal}
                />
              )}
            </>
          )}

          {/* All accounts view (no filter) */}
          {!selectedSchool && !filterOrgId && !allAccountsLoading && (
            <>
              {allAccounts?.length === 0 && (
                <SimpleEmptyState
                  title="Keine Konten"
                  description="Es gibt noch keine Konten im System."
                />
              )}
              {allAccounts && allAccounts.length > 0 && (
                <AccountsTable
                  accounts={allAccounts}
                  showSchool
                  onManageCaregiver={openCaregiverModal}
                />
              )}
            </>
          )}

          {/* School-level view */}
          {selectedSchool && !schoolAccountsLoading && (
            <>
              <div className="mb-3 flex items-center gap-2">
                <h3 className="text-sm font-semibold text-gray-900">
                  {selectedSchool.name}
                </h3>
                {selectedSchool.organization && (
                  <span className="text-xs text-gray-400">
                    {selectedSchool.organization.name}
                  </span>
                )}
                {schoolAccounts && (
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
                    {schoolAccounts.length}{" "}
                    {schoolAccounts.length === 1 ? "Konto" : "Konten"}
                  </span>
                )}
              </div>
              {schoolAccounts?.length === 0 && (
                <SimpleEmptyState
                  title="Keine Konten"
                  description="Für diese Schule gibt es noch keine zugewiesenen Konten."
                />
              )}
              {schoolAccounts && schoolAccounts.length > 0 && (
                <AccountsTable
                  accounts={schoolAccounts}
                  selectedSchool={selectedSchool}
                  onManageCaregiver={openCaregiverModal}
                />
              )}
            </>
          )}
        </>
      )}

      {/* Devices Tab */}
      {activeTab === "devices" && (
        <>
          <div className="mb-4 flex items-center justify-between">
            <div />
            <button
              type="button"
              onClick={() => setCreateDeviceOpen(true)}
              className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-800"
            >
              <PlusIcon />
              Neues Gerät
            </button>
          </div>
          <OrgSchoolFilter
            idPrefix="device"
            organizations={organizations}
            filteredSchools={filteredSchools}
            filterOrgId={filterOrgId}
            selectedSchool={selectedSchool}
            onOrgChange={handleOrgFilterChange}
            onSchoolChange={handleSchoolFilterChange}
          />

          {/* Loading */}
          {devicesLoading && <CardSkeletons />}

          {/* Org-level view (no specific school selected) */}
          {!selectedSchool && filterOrgId && !orgDevicesLoading && (
            <>
              {orgDevices?.length === 0 && (
                <SimpleEmptyState
                  title="Keine Geräte"
                  description="Für diesen Träger gibt es noch keine registrierten Geräte."
                />
              )}
              {orgDevices && orgDevices.length > 0 && (
                <DevicesTable
                  devices={orgDevices}
                  showSchool
                  onSetKey={setSetKeyDevice}
                  onDelete={setDeleteDevice}
                />
              )}
            </>
          )}

          {/* All devices view (no filter) */}
          {!selectedSchool && !filterOrgId && !allDevicesLoading && (
            <>
              {allDevices?.length === 0 && (
                <SimpleEmptyState
                  title="Keine Geräte"
                  description="Es gibt noch keine registrierten Geräte im System."
                />
              )}
              {allDevices && allDevices.length > 0 && (
                <DevicesTable
                  devices={allDevices}
                  showSchool
                  onSetKey={setSetKeyDevice}
                  onDelete={setDeleteDevice}
                />
              )}
            </>
          )}

          {/* School-level view */}
          {selectedSchool && !schoolDevicesLoading && (
            <>
              <div className="mb-3 flex items-center gap-2">
                <h3 className="text-sm font-semibold text-gray-900">
                  {selectedSchool.name}
                </h3>
                {selectedSchool.organization && (
                  <span className="text-xs text-gray-400">
                    {selectedSchool.organization.name}
                  </span>
                )}
                {schoolDevices && (
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
                    {schoolDevices.length}{" "}
                    {schoolDevices.length === 1 ? "Gerät" : "Geräte"}
                  </span>
                )}
              </div>
              {schoolDevices?.length === 0 && (
                <SimpleEmptyState
                  title="Keine Geräte"
                  description="Für diese Schule gibt es noch keine registrierten Geräte."
                />
              )}
              {schoolDevices && schoolDevices.length > 0 && (
                <DevicesTable
                  devices={schoolDevices}
                  onSetKey={setSetKeyDevice}
                  onDelete={setDeleteDevice}
                />
              )}
            </>
          )}
        </>
      )}

      {/* Persons Tab */}
      {activeTab === "persons" && (
        <>
          {!selectedSchool ? (
            <SimpleEmptyState
              title="Keine Schule ausgewählt"
              description="Wählen Sie eine Schule aus, um deren Personen anzuzeigen."
            />
          ) : schoolPersonsLoading ? (
            <CardSkeletons />
          ) : !schoolPersons || schoolPersons.length === 0 ? (
            <SimpleEmptyState
              title="Keine Personen"
              description={`Keine Personen in ${selectedSchool.name} vorhanden.`}
            />
          ) : (
            <>
              <p className="mb-4 text-sm text-gray-500">
                {schoolPersons.length}{" "}
                {schoolPersons.length === 1 ? "Person" : "Personen"} in{" "}
                <span className="font-medium">{selectedSchool.name}</span>
              </p>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {schoolPersons.map((person) => (
                  <div
                    key={person.id}
                    className="flex items-center justify-between rounded-xl border border-gray-100 bg-white p-4 shadow-sm"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium text-gray-900">
                        {person.firstName} {person.lastName}
                      </p>
                      <div className="mt-1 flex flex-wrap gap-1.5">
                        {person.isStaff && (
                          <span className="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">
                            Mitarbeiter
                          </span>
                        )}
                        {person.isStudent && (
                          <span className="inline-flex items-center rounded-full bg-green-50 px-2 py-0.5 text-xs font-medium text-green-700">
                            Schüler
                          </span>
                        )}
                        {person.hasRfidCard && (
                          <span className="inline-flex items-center rounded-full bg-purple-50 px-2 py-0.5 text-xs font-medium text-purple-700">
                            RFID
                          </span>
                        )}
                      </div>
                      {person.accountEmail && (
                        <p className="mt-1 truncate text-xs text-gray-500">
                          {person.accountEmail}
                        </p>
                      )}
                    </div>
                    <button
                      type="button"
                      onClick={() => setDeletePersonTarget(person)}
                      className="ml-3 shrink-0 rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600"
                      title="Person löschen"
                    >
                      <svg
                        className="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                        strokeWidth={2}
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                        />
                      </svg>
                    </button>
                  </div>
                ))}
              </div>
            </>
          )}
        </>
      )}

      {/* Person Delete Triple-Confirm Dialog */}
      {deletePersonTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h3 className="text-lg font-semibold text-gray-900">
              Person löschen
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              Möchten Sie{" "}
              <span className="font-medium">
                {deletePersonTarget.firstName} {deletePersonTarget.lastName}
              </span>{" "}
              von{" "}
              <span className="font-medium">
                {deletePersonTarget.schoolName}
              </span>{" "}
              wirklich löschen?
            </p>
            <div className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
              <p className="font-medium">
                Folgende Aktionen werden ausgeführt:
              </p>
              <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
                <li>Account wird deaktiviert und Login gesperrt</li>
                <li>Persönliche Daten werden anonymisiert</li>
                <li>RFID-Karte wird freigegeben</li>
                <li>Diese Aktion kann nicht rückgängig gemacht werden</li>
              </ul>
            </div>

            <div className="mt-4">
              <label
                htmlFor="delete-person-confirm"
                className="block text-sm font-medium text-gray-700"
              >
                Geben Sie den vollständigen Namen ein:
              </label>
              <p className="mb-1 text-sm font-medium text-gray-900">
                {deletePersonTarget.fullName}
              </p>
              <input
                id="delete-person-confirm"
                type="text"
                value={deletePersonConfirmInput}
                onChange={(e) => setDeletePersonConfirmInput(e.target.value)}
                placeholder={deletePersonTarget.fullName}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-red-500 focus:ring-1 focus:ring-red-500 focus:outline-none"
                autoComplete="off"
              />
            </div>

            {deletePersonError && (
              <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
                {deletePersonError}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => setDeletePersonTarget(null)}
                disabled={deletePersonLoading}
                className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
              >
                Abbrechen
              </button>
              <button
                type="button"
                onClick={() => void handleDeletePerson()}
                disabled={
                  deletePersonLoading ||
                  deletePersonConfirmInput !== deletePersonTarget.fullName
                }
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {deletePersonLoading ? "Wird gelöscht..." : "Endgültig löschen"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modals */}
      <CreateOrganizationModal
        isOpen={createOrgOpen}
        onClose={() => setCreateOrgOpen(false)}
        onCreated={() => mutateOrgs().then(() => undefined)}
      />
      <EditOrganizationModal
        isOpen={editOrgOpen}
        onClose={() => {
          setEditOrgOpen(false);
          setEditOrgTarget(null);
        }}
        organization={editOrgTarget}
        onUpdated={() => mutateOrgs().then(() => undefined)}
      />
      <CreateSchoolModal
        isOpen={createSchoolOpen}
        onClose={() => setCreateSchoolOpen(false)}
        organizations={organizations}
        onCreated={() => mutateSchools().then(() => undefined)}
      />
      <EditSchoolModal
        isOpen={editSchoolOpen}
        onClose={() => {
          setEditSchoolOpen(false);
          setEditSchoolTarget(null);
        }}
        school={editSchoolTarget}
        organizations={organizations}
        onUpdated={() => mutateSchools().then(() => undefined)}
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
      <CreateDeviceModal
        isOpen={createDeviceOpen}
        onClose={() => setCreateDeviceOpen(false)}
        schools={schools}
        onCreated={() => {
          void refreshDevices();
        }}
      />
      <SetApiKeyModal
        isOpen={setKeyDevice !== null}
        onClose={() => setSetKeyDevice(null)}
        device={setKeyDevice}
        onKeySet={() => {
          void refreshDevices();
        }}
      />
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
            await refreshAccounts();
          }}
        />
      ) : null}

      {/* Delete Device Confirmation Dialog */}
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

// --- Card Sub-components (tightly coupled to the card list) ---

function OrganizationCard({
  organization,
  onEdit,
  onToggleActive,
}: {
  readonly organization: Organization;
  readonly onEdit: (org: Organization) => void;
  readonly onToggleActive: (org: Organization) => Promise<void>;
}) {
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="text-base font-semibold text-gray-900">
            {organization.name}
          </h3>
          <p className="mt-0.5 font-mono text-sm text-gray-500">
            {organization.slug}
          </p>
        </div>
        <button
          type="button"
          onClick={() => void onToggleActive(organization)}
          className="cursor-pointer"
          title={organization.active ? "Deaktivieren" : "Aktivieren"}
          aria-label={organization.active ? "Deaktivieren" : "Aktivieren"}
        >
          <StatusBadge active={organization.active} />
        </button>
      </div>
      <div className="mt-3 flex items-center justify-between">
        <p className="text-xs text-gray-400">
          Erstellt {getRelativeTime(organization.createdAt)}
        </p>
        <button
          type="button"
          onClick={() => onEdit(organization)}
          className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
        >
          Bearbeiten
        </button>
      </div>
    </div>
  );
}

function DeletedSchoolCard({
  school,
  onRestore,
}: {
  readonly school: School;
  readonly onRestore: (school: School) => void;
}) {
  return (
    <div className="rounded-3xl border border-red-100/50 bg-red-50/50 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.08)]">
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <h3 className="text-base font-semibold text-gray-900">
            {school.name}
          </h3>
          <div className="mt-0.5 flex items-center gap-2 text-sm text-gray-500">
            <span className="font-mono">{school.subdomain}</span>
            {school.organization && (
              <>
                <span className="text-gray-300">·</span>
                <span>{school.organization.name}</span>
              </>
            )}
          </div>
        </div>
        <span className="rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700">
          Gelöscht
        </span>
      </div>
      <div className="mt-3 flex items-center justify-between">
        <p className="text-xs text-gray-400">
          Gelöscht {school.deletedAt ? getRelativeTime(school.deletedAt) : ""}
        </p>
        <button
          type="button"
          onClick={() => onRestore(school)}
          className="rounded-lg bg-green-100 px-3 py-1.5 text-xs font-medium text-green-700 transition-colors hover:bg-green-200"
        >
          Wiederherstellen
        </button>
      </div>
    </div>
  );
}

function SchoolCard({
  school,
  onEdit,
  onToggleActive,
  onInviteAdmin,
  onCreateAccount,
  onViewAccounts,
  onDelete,
}: {
  readonly school: School;
  readonly onEdit: (school: School) => void;
  readonly onToggleActive: (school: School) => Promise<void>;
  readonly onInviteAdmin: (school: School) => void;
  readonly onCreateAccount: (school: School) => void;
  readonly onViewAccounts: (school: School) => void;
  readonly onDelete: (school: School) => void;
}) {
  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-150">
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <h3 className="text-base font-semibold text-gray-900">
            {school.name}
          </h3>
          <div className="mt-0.5 flex items-center gap-2 text-sm text-gray-500">
            <span className="font-mono">{school.subdomain}</span>
            {school.organization && (
              <>
                <span className="text-gray-300">·</span>
                <span>{school.organization.name}</span>
              </>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {school.hidden && (
            <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700">
              Verborgen
            </span>
          )}
          <button
            type="button"
            onClick={() => void onToggleActive(school)}
            className="cursor-pointer"
            title={school.active ? "Deaktivieren" : "Aktivieren"}
            aria-label={school.active ? "Deaktivieren" : "Aktivieren"}
          >
            <StatusBadge active={school.active} />
          </button>
        </div>
      </div>

      {(school.address || school.city) && (
        <p className="mt-2 text-xs text-gray-400">
          {[school.address, school.zip, school.city].filter(Boolean).join(", ")}
        </p>
      )}

      <div className="mt-3 flex items-center justify-between">
        <p className="text-xs text-gray-400">
          Erstellt {getRelativeTime(school.createdAt)}
        </p>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => onViewAccounts(school)}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Konten
          </button>
          <button
            type="button"
            onClick={() => onEdit(school)}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Bearbeiten
          </button>
          <button
            type="button"
            onClick={() => onCreateAccount(school)}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Konto erstellen
          </button>
          <button
            type="button"
            onClick={() => onInviteAdmin(school)}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Admin einladen
          </button>
          <button
            type="button"
            onClick={() => onDelete(school)}
            className="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-100"
          >
            Löschen
          </button>
        </div>
      </div>
    </div>
  );
}

type SortDirection = "asc" | "desc";

interface SortState<K extends string> {
  key: K;
  direction: SortDirection;
}

function useSort<K extends string>(
  defaultKey: K,
  defaultDirection: SortDirection = "asc",
) {
  const [sort, setSort] = useState<SortState<K>>({
    key: defaultKey,
    direction: defaultDirection,
  });

  const toggle = useCallback((key: K) => {
    setSort((prev) =>
      prev.key === key
        ? { key, direction: prev.direction === "asc" ? "desc" : "asc" }
        : { key, direction: "asc" },
    );
  }, []);

  return { sort, toggle };
}

function SortIndicator({
  active,
  direction,
}: {
  readonly active: boolean;
  readonly direction: SortDirection;
}) {
  return (
    <span
      className={`ml-1 inline-block ${active ? "text-gray-700" : "text-gray-300"}`}
    >
      {active ? (direction === "asc" ? "↑" : "↓") : "↕"}
    </span>
  );
}

function SortableHeader<K extends string>({
  label,
  sortKey,
  sort,
  onToggle,
}: {
  readonly label: string;
  readonly sortKey: K;
  readonly sort: SortState<K>;
  readonly onToggle: (key: K) => void;
}) {
  return (
    <th
      className="cursor-pointer px-5 py-3 transition-colors select-none hover:text-gray-700"
      onClick={() => onToggle(sortKey)}
    >
      {label}
      <SortIndicator active={sort.key === sortKey} direction={sort.direction} />
    </th>
  );
}

function getAccountSortValue(account: SchoolAccount, key: string): string {
  switch (key) {
    case "name":
      return account.firstName || account.lastName
        ? `${account.lastName} ${account.firstName}`.trim().toLowerCase()
        : "";
    case "email":
      return account.email.toLowerCase();
    case "roleName":
      return account.roleName.toLowerCase();
    case "pedagogicRole":
      return account.pedagogicRole.toLowerCase();
    case "status":
      return account.status.toLowerCase();
    case "schoolName":
      return "schoolName" in account
        ? (account as OrgAccount).schoolName.toLowerCase()
        : "";
    default:
      return "";
  }
}

type AccountSortKey =
  | "name"
  | "email"
  | "roleName"
  | "pedagogicRole"
  | "status"
  | "schoolName";

function sortAccounts<T extends SchoolAccount>(
  accounts: readonly T[],
  sort: SortState<AccountSortKey>,
): T[] {
  const dir = sort.direction === "asc" ? 1 : -1;
  return [...accounts].sort((a, b) => {
    const av = getAccountSortValue(a, sort.key);
    const bv = getAccountSortValue(b, sort.key);
    return av < bv ? -dir : av > bv ? dir : 0;
  });
}

function getAccountCapabilityMeta(account: SchoolAccount) {
  if (account.isActiveCaregiver && account.hasAdminRole) {
    return {
      label: "Verwaltung + Betreuung",
      className: "bg-emerald-100 text-emerald-700",
    };
  }
  if (account.isActiveCaregiver) {
    return {
      label: "Betreuung aktiv",
      className: "bg-orange-100 text-orange-700",
    };
  }
  if (account.hasUserRole && !account.hasCaregiverProfile) {
    return {
      label: "Betreuung unvollständig",
      className: "bg-amber-100 text-amber-700",
    };
  }
  if (account.hasCaregiverProfile && !account.hasUserRole) {
    return {
      label: "Betreuungsprofil inaktiv",
      className: "bg-slate-100 text-slate-700",
    };
  }
  if (account.hasAdminRole) {
    return {
      label: "Nur Verwaltung",
      className: "bg-blue-100 text-blue-700",
    };
  }
  return {
    label: "Keine Betreuung",
    className: "bg-gray-100 text-gray-600",
  };
}

function AccountRowWithAction({
  account,
  schoolContext,
  onManageCaregiver,
}: {
  readonly account: SchoolAccount | OrgAccount;
  readonly schoolContext: { id: string; name: string } | null;
  readonly onManageCaregiver?: (
    account: SchoolAccount | OrgAccount,
    schoolContext: { id: string; name: string } | null,
  ) => void;
}) {
  const capability = getAccountCapabilityMeta(account);
  const canManageCaregiver =
    account.accountId !== "0" && account.status !== "invited";

  return (
    <>
      <td className="px-5 py-3 font-medium text-gray-900">
        {account.firstName || account.lastName
          ? `${account.firstName} ${account.lastName}`.trim()
          : "—"}
      </td>
      <td className="px-5 py-3 text-gray-600">{account.email}</td>
      <td className="px-5 py-3">
        {account.roleName ? (
          <span className="inline-flex flex-wrap gap-1">
            {account.roleName.split(", ").map((role) => (
              <span
                key={role}
                className="inline-flex rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700"
              >
                {role}
              </span>
            ))}
          </span>
        ) : (
          <span className="text-gray-400">—</span>
        )}
      </td>
      <td className="px-5 py-3 text-gray-600">
        {account.pedagogicRole || "—"}
      </td>
      <td className="px-5 py-3">
        <span
          className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${capability.className}`}
        >
          {capability.label}
        </span>
      </td>
      <td className="px-5 py-3">
        <AccountStatusBadge status={account.status} />
      </td>
      <td className="px-5 py-3 text-right">
        {canManageCaregiver ? (
          <button
            type="button"
            onClick={() => onManageCaregiver?.(account, schoolContext)}
            className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Betreuung verwalten
          </button>
        ) : (
          <span className="text-xs text-gray-400">—</span>
        )}
      </td>
    </>
  );
}

function AccountsTable({
  accounts,
  showSchool = false,
  selectedSchool,
  onManageCaregiver,
}: {
  readonly accounts: SchoolAccount[] | OrgAccount[];
  readonly showSchool?: boolean;
  readonly selectedSchool?: School | null;
  readonly onManageCaregiver?: (
    account: SchoolAccount | OrgAccount,
    schoolContext: { id: string; name: string } | null,
  ) => void;
}) {
  const { sort, toggle } = useSort<AccountSortKey>(
    showSchool ? "schoolName" : "name",
  );
  const sorted = useMemo(() => sortAccounts(accounts, sort), [accounts, sort]);

  return (
    <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
            {showSchool && (
              <SortableHeader
                label="Schule"
                sortKey="schoolName"
                sort={sort}
                onToggle={toggle}
              />
            )}
            <SortableHeader
              label="Name"
              sortKey="name"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="E-Mail"
              sortKey="email"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Rolle"
              sortKey="roleName"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Päd. Rolle"
              sortKey="pedagogicRole"
              sort={sort}
              onToggle={toggle}
            />
            <th className="px-5 py-3">Einsatz</th>
            <SortableHeader
              label="Status"
              sortKey="status"
              sort={sort}
              onToggle={toggle}
            />
            <th className="px-5 py-3 text-right">Aktion</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((account) => {
            const orgAccount = showSchool ? (account as OrgAccount) : undefined;
            const schoolContext = showSchool
              ? orgAccount
                ? { id: orgAccount.schoolId, name: orgAccount.schoolName }
                : null
              : selectedSchool
                ? { id: selectedSchool.id, name: selectedSchool.name }
                : null;
            return (
              <tr
                key={
                  account.accountId !== "0"
                    ? `${orgAccount?.schoolId ?? ""}-${account.accountId}`
                    : `${orgAccount?.schoolId ?? ""}-invited-${account.email}`
                }
                className="border-b border-gray-50 last:border-0"
              >
                {showSchool && orgAccount && (
                  <td className="px-5 py-3 text-gray-600">
                    {orgAccount.schoolName}
                  </td>
                )}
                <AccountRowWithAction
                  account={account}
                  schoolContext={schoolContext}
                  onManageCaregiver={onManageCaregiver}
                />
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function OrgSchoolFilter({
  idPrefix,
  organizations,
  filteredSchools,
  filterOrgId,
  selectedSchool,
  onOrgChange,
  onSchoolChange,
}: {
  readonly idPrefix: string;
  readonly organizations: Organization[] | undefined;
  readonly filteredSchools: School[];
  readonly filterOrgId: string;
  readonly selectedSchool: School | null;
  readonly onOrgChange: (orgId: string) => void;
  readonly onSchoolChange: (schoolId: string) => void;
}) {
  return (
    <div className="mt-4 mb-4 flex flex-col gap-3 sm:flex-row sm:items-end">
      <div className="flex-1">
        <label
          htmlFor={`filter-${idPrefix}-org`}
          className="mb-1 block text-xs font-medium text-gray-500"
        >
          Träger
        </label>
        <select
          id={`filter-${idPrefix}-org`}
          value={filterOrgId}
          onChange={(e) => onOrgChange(e.target.value)}
          className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-1 focus:ring-gray-400 focus:outline-none"
        >
          <option value="">Alle Träger</option>
          {organizations?.map((org) => (
            <option key={org.id} value={org.id}>
              {org.name}
            </option>
          ))}
        </select>
      </div>
      <div className="flex-1">
        <label
          htmlFor={`filter-${idPrefix}-school`}
          className="mb-1 block text-xs font-medium text-gray-500"
        >
          Schule
        </label>
        <select
          id={`filter-${idPrefix}-school`}
          value={selectedSchool?.id ?? ""}
          onChange={(e) => onSchoolChange(e.target.value)}
          className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-1 focus:ring-gray-400 focus:outline-none"
        >
          <option value="">
            {filterOrgId ? "Alle Schulen" : "Schule auswählen…"}
          </option>
          {filteredSchools.map((school) => (
            <option key={school.id} value={school.id}>
              {school.name}
              {school.organization ? ` (${school.organization.name})` : ""}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}

function AccountStatusBadge({ status }: { readonly status: string }) {
  const styles: Record<string, string> = {
    active: "bg-green-100 text-green-700",
    pending: "bg-yellow-100 text-yellow-700",
    invited: "bg-purple-100 text-purple-700",
    inactive: "bg-gray-100 text-gray-500",
  };
  const labels: Record<string, string> = {
    active: "Aktiv",
    pending: "Ausstehend",
    invited: "Eingeladen",
    inactive: "Inaktiv",
  };
  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-gray-100 text-gray-500"}`}
    >
      {labels[status] ?? status}
    </span>
  );
}

// --- Device Tab Components ---

type DeviceSortKey =
  | "schoolName"
  | "deviceId"
  | "deviceType"
  | "name"
  | "status"
  | "lastSeen"
  | "apiKey";

function sortDevices(
  devices: readonly OperatorDevice[],
  sort: SortState<DeviceSortKey>,
): OperatorDevice[] {
  const dir = sort.direction === "asc" ? 1 : -1;
  return [...devices].sort((a, b) => {
    let av: string;
    let bv: string;
    switch (sort.key) {
      case "schoolName":
        av = a.schoolName.toLowerCase();
        bv = b.schoolName.toLowerCase();
        break;
      case "deviceId":
        av = a.deviceId.toLowerCase();
        bv = b.deviceId.toLowerCase();
        break;
      case "deviceType":
        av = a.deviceType.toLowerCase();
        bv = b.deviceType.toLowerCase();
        break;
      case "name":
        av = (a.name || "").toLowerCase();
        bv = (b.name || "").toLowerCase();
        break;
      case "status":
        av = a.status.toLowerCase();
        bv = b.status.toLowerCase();
        break;
      case "lastSeen":
        av = a.lastSeen ?? "";
        bv = b.lastSeen ?? "";
        break;
      case "apiKey":
        av = a.maskedApiKey.toLowerCase();
        bv = b.maskedApiKey.toLowerCase();
        break;
    }
    return av < bv ? -dir : av > bv ? dir : 0;
  });
}

function DevicesTable({
  devices,
  showSchool = false,
  onSetKey,
  onDelete,
}: {
  readonly devices: OperatorDevice[];
  readonly showSchool?: boolean;
  readonly onSetKey?: (device: OperatorDevice) => void;
  readonly onDelete?: (device: OperatorDevice) => void;
}) {
  const { sort, toggle } = useSort<DeviceSortKey>(
    showSchool ? "schoolName" : "deviceId",
  );
  const sorted = useMemo(() => sortDevices(devices, sort), [devices, sort]);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const handleCopyApiKey = useCallback(async (device: OperatorDevice) => {
    if (!device.apiKey) return;
    try {
      await navigator.clipboard.writeText(device.apiKey);
      setCopiedId(device.id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      logger.error("clipboard_copy_failed", {
        error: "Failed to copy API key to clipboard",
      });
    }
  }, []);

  return (
    <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
            {showSchool && (
              <SortableHeader
                label="Schule"
                sortKey="schoolName"
                sort={sort}
                onToggle={toggle}
              />
            )}
            <SortableHeader
              label="Geräte-ID"
              sortKey="deviceId"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Name"
              sortKey="name"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Typ"
              sortKey="deviceType"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Status"
              sortKey="status"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Zuletzt online"
              sortKey="lastSeen"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="API-Key"
              sortKey="apiKey"
              sort={sort}
              onToggle={toggle}
            />
            {(onSetKey || onDelete) && (
              <th className="px-5 py-3 text-xs font-medium text-gray-500">
                Aktionen
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {sorted.map((device) => (
            <tr
              key={device.id}
              className="border-b border-gray-50 last:border-0"
            >
              {showSchool && (
                <td className="px-5 py-3 text-gray-600">{device.schoolName}</td>
              )}
              <td className="px-5 py-3 font-mono text-xs font-medium text-gray-900">
                {device.deviceId}
              </td>
              <td className="px-5 py-3 text-gray-600">{device.name || "—"}</td>
              <td className="px-5 py-3">
                <span className="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
                  {getDeviceTypeDisplayName(device.deviceType)}
                </span>
              </td>
              <td className="px-5 py-3">
                <DeviceStatusBadge
                  status={device.status}
                  isOnline={device.isOnline}
                />
              </td>
              <td className="px-5 py-3 text-gray-600">
                {device.lastSeen ? (
                  <span title={formatLastSeen(device.lastSeen)}>
                    {getRelativeTime(device.lastSeen)}
                  </span>
                ) : (
                  <span className="text-gray-400">Nie</span>
                )}
              </td>
              <td className="px-5 py-3">
                {device.maskedApiKey ? (
                  <button
                    type="button"
                    onClick={() => void handleCopyApiKey(device)}
                    className="group flex items-center gap-1.5 font-mono text-xs text-gray-500 transition-colors hover:text-gray-900"
                    title="API-Key kopieren"
                  >
                    <span>{device.maskedApiKey}</span>
                    <span className="text-gray-300 transition-colors group-hover:text-gray-600">
                      {copiedId === device.id ? <CheckIcon /> : <CopyIcon />}
                    </span>
                  </button>
                ) : (
                  <span className="text-gray-400">—</span>
                )}
              </td>
              {(onSetKey || onDelete) && (
                <td className="px-5 py-3">
                  <div className="flex items-center gap-2">
                    {onSetKey && (
                      <button
                        type="button"
                        onClick={() => onSetKey(device)}
                        className="rounded-lg border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
                        title="API-Key ändern"
                      >
                        Key ändern
                      </button>
                    )}
                    {onDelete && (
                      <button
                        type="button"
                        onClick={() => onDelete(device)}
                        className="rounded-lg border border-red-200 px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 hover:text-red-700"
                        title="Gerät löschen"
                      >
                        Löschen
                      </button>
                    )}
                  </div>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DeviceStatusBadge({
  status,
  isOnline,
}: {
  readonly status: string;
  readonly isOnline: boolean;
}) {
  if (isOnline) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">
        <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
        Online
      </span>
    );
  }
  const styles: Record<string, string> = {
    active: "bg-green-100 text-green-700",
    inactive: "bg-gray-100 text-gray-500",
    maintenance: "bg-yellow-100 text-yellow-700",
    offline: "bg-red-100 text-red-700",
  };
  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-gray-100 text-gray-500"}`}
    >
      {getDeviceStatusDisplayName(status)}
    </span>
  );
}

function CopyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
      <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="text-green-600"
    >
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}
