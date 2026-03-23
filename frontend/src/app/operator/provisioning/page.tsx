"use client";

import { useState, useMemo, useCallback } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
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
  PlusIcon,
  CardSkeletons,
} from "./provisioning-shared";
import { CreateOrganizationModal } from "./create-organization-modal";
import { EditOrganizationModal } from "./edit-organization-modal";
import { CreateSchoolModal } from "./create-school-modal";
import { EditSchoolModal } from "./edit-school-modal";
import { InviteAdminModal } from "./invite-admin-modal";

const logger = createLogger({ component: "OperatorProvisioningPage" });

type ActiveTab = "organizations" | "schools" | "accounts" | "devices";

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
  const [selectedSchool, setSelectedSchool] = useState<School | null>(null);
  const [filterOrgId, setFilterOrgId] = useState<string>("");

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
        await operatorProvisioningService.updateSchool(school.id, {
          organization_id: parseInt(school.organizationId, 10),
          name: school.name,
          slug: school.slug,
          subdomain: school.subdomain,
          address: school.address ?? "",
          city: school.city ?? "",
          zip: school.zip ?? "",
          phone: school.phone ?? "",
          email: school.email ?? "",
          active: !school.active,
        });
        await mutateSchools();
        try {
          await fetch("/api/operator/provisioning/revalidate-tenant", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ slugs: [school.subdomain] }),
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
          {schools?.length === 0 && (
            <EmptyState
              title="Keine Schulen"
              description="Erstellen Sie eine neue Schule unter einem Träger."
              buttonLabel="Neue Schule"
              onAction={() => setCreateSchoolOpen(true)}
            />
          )}
          {schools && schools.length > 0 && (
            <div className="mt-4 space-y-4">
              {schools.map((school) => (
                <SchoolCard
                  key={school.id}
                  school={school}
                  onEdit={openEditSchool}
                  onToggleActive={handleToggleSchoolActive}
                  onInviteAdmin={openInviteAdmin}
                  onViewAccounts={openSchoolAccounts}
                />
              ))}
            </div>
          )}
          {schoolToggleError && (
            <p className="mt-2 text-sm text-red-600">{schoolToggleError}</p>
          )}
        </>
      )}

      {/* Accounts Tab */}
      {activeTab === "accounts" && (
        <>
          {/* Filters */}
          <div className="mt-4 mb-4 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <label
                htmlFor="filter-org"
                className="mb-1 block text-xs font-medium text-gray-500"
              >
                Träger
              </label>
              <select
                id="filter-org"
                value={filterOrgId}
                onChange={(e) => {
                  setFilterOrgId(e.target.value);
                  // Reset school selection when org changes
                  const newOrgId = e.target.value;
                  if (
                    selectedSchool &&
                    newOrgId &&
                    selectedSchool.organizationId !== newOrgId
                  ) {
                    setSelectedSchool(null);
                  }
                }}
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
                htmlFor="filter-school"
                className="mb-1 block text-xs font-medium text-gray-500"
              >
                Schule
              </label>
              <select
                id="filter-school"
                value={selectedSchool?.id ?? ""}
                onChange={(e) => {
                  const schoolId = e.target.value;
                  if (!schoolId) {
                    setSelectedSchool(null);
                    return;
                  }
                  const school = schools?.find((s) => s.id === schoolId);
                  if (school) {
                    setSelectedSchool(school);
                  }
                }}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-1 focus:ring-gray-400 focus:outline-none"
              >
                <option value="">
                  {filterOrgId ? "Alle Schulen" : "Schule auswählen…"}
                </option>
                {filteredSchools.map((school) => (
                  <option key={school.id} value={school.id}>
                    {school.name}
                    {school.organization
                      ? ` (${school.organization.name})`
                      : ""}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Loading */}
          {accountsLoading && <CardSkeletons />}

          {/* Org-level view (no specific school selected) */}
          {!selectedSchool && filterOrgId && !orgAccountsLoading && (
            <>
              {orgAccounts?.length === 0 && (
                <div className="flex flex-col items-center gap-3 py-12 text-center">
                  <p className="text-lg font-medium text-gray-900">
                    Keine Konten
                  </p>
                  <p className="text-sm text-gray-500">
                    Für diesen Träger gibt es noch keine zugewiesenen Konten.
                  </p>
                </div>
              )}
              {orgAccounts && orgAccounts.length > 0 && (
                <OrgAccountsTable accounts={orgAccounts} />
              )}
            </>
          )}

          {/* All accounts view (no filter) */}
          {!selectedSchool && !filterOrgId && !allAccountsLoading && (
            <>
              {allAccounts?.length === 0 && (
                <div className="flex flex-col items-center gap-3 py-12 text-center">
                  <p className="text-lg font-medium text-gray-900">
                    Keine Konten
                  </p>
                  <p className="text-sm text-gray-500">
                    Es gibt noch keine Konten im System.
                  </p>
                </div>
              )}
              {allAccounts && allAccounts.length > 0 && (
                <OrgAccountsTable accounts={allAccounts} />
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
                <div className="flex flex-col items-center gap-3 py-12 text-center">
                  <p className="text-lg font-medium text-gray-900">
                    Keine Konten
                  </p>
                  <p className="text-sm text-gray-500">
                    Für diese Schule gibt es noch keine zugewiesenen Konten.
                  </p>
                </div>
              )}
              {schoolAccounts && schoolAccounts.length > 0 && (
                <AccountsTable accounts={schoolAccounts} />
              )}
            </>
          )}
        </>
      )}

      {/* Devices Tab */}
      {activeTab === "devices" && (
        <>
          {/* Filters */}
          <div className="mt-4 mb-4 flex flex-col gap-3 sm:flex-row sm:items-end">
            <div className="flex-1">
              <label
                htmlFor="filter-device-org"
                className="mb-1 block text-xs font-medium text-gray-500"
              >
                Träger
              </label>
              <select
                id="filter-device-org"
                value={filterOrgId}
                onChange={(e) => {
                  setFilterOrgId(e.target.value);
                  const newOrgId = e.target.value;
                  if (
                    selectedSchool &&
                    newOrgId &&
                    selectedSchool.organizationId !== newOrgId
                  ) {
                    setSelectedSchool(null);
                  }
                }}
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
                htmlFor="filter-device-school"
                className="mb-1 block text-xs font-medium text-gray-500"
              >
                Schule
              </label>
              <select
                id="filter-device-school"
                value={selectedSchool?.id ?? ""}
                onChange={(e) => {
                  const schoolId = e.target.value;
                  if (!schoolId) {
                    setSelectedSchool(null);
                    return;
                  }
                  const school = schools?.find((s) => s.id === schoolId);
                  if (school) {
                    setSelectedSchool(school);
                  }
                }}
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:ring-1 focus:ring-gray-400 focus:outline-none"
              >
                <option value="">
                  {filterOrgId ? "Alle Schulen" : "Schule auswählen…"}
                </option>
                {filteredSchools.map((school) => (
                  <option key={school.id} value={school.id}>
                    {school.name}
                    {school.organization
                      ? ` (${school.organization.name})`
                      : ""}
                  </option>
                ))}
              </select>
            </div>
          </div>

          {/* Loading */}
          {devicesLoading && <CardSkeletons />}

          {/* Org-level view (no specific school selected) */}
          {!selectedSchool && filterOrgId && !orgDevicesLoading && (
            <>
              {orgDevices?.length === 0 && (
                <div className="flex flex-col items-center gap-3 py-12 text-center">
                  <p className="text-lg font-medium text-gray-900">
                    Keine Geräte
                  </p>
                  <p className="text-sm text-gray-500">
                    Für diesen Träger gibt es noch keine registrierten Geräte.
                  </p>
                </div>
              )}
              {orgDevices && orgDevices.length > 0 && (
                <DevicesTable devices={orgDevices} showSchool />
              )}
            </>
          )}

          {/* All devices view (no filter) */}
          {!selectedSchool && !filterOrgId && !allDevicesLoading && (
            <>
              {allDevices?.length === 0 && (
                <div className="flex flex-col items-center gap-3 py-12 text-center">
                  <p className="text-lg font-medium text-gray-900">
                    Keine Geräte
                  </p>
                  <p className="text-sm text-gray-500">
                    Es gibt noch keine registrierten Geräte im System.
                  </p>
                </div>
              )}
              {allDevices && allDevices.length > 0 && (
                <DevicesTable devices={allDevices} showSchool />
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
                <div className="flex flex-col items-center gap-3 py-12 text-center">
                  <p className="text-lg font-medium text-gray-900">
                    Keine Geräte
                  </p>
                  <p className="text-sm text-gray-500">
                    Für diese Schule gibt es noch keine registrierten Geräte.
                  </p>
                </div>
              )}
              {schoolDevices && schoolDevices.length > 0 && (
                <DevicesTable devices={schoolDevices} />
              )}
            </>
          )}
        </>
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

function SchoolCard({
  school,
  onEdit,
  onToggleActive,
  onInviteAdmin,
  onViewAccounts,
}: {
  readonly school: School;
  readonly onEdit: (school: School) => void;
  readonly onToggleActive: (school: School) => Promise<void>;
  readonly onInviteAdmin: (school: School) => void;
  readonly onViewAccounts: (school: School) => void;
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
            onClick={() => onInviteAdmin(school)}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Admin einladen
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

function getAccountName(account: SchoolAccount): string {
  return account.firstName || account.lastName
    ? `${account.lastName} ${account.firstName}`.trim().toLowerCase()
    : "";
}

type SchoolAccountSortKey =
  | "name"
  | "email"
  | "roleName"
  | "pedagogicRole"
  | "status";

function sortSchoolAccounts(
  accounts: readonly SchoolAccount[],
  sort: SortState<SchoolAccountSortKey>,
): SchoolAccount[] {
  const dir = sort.direction === "asc" ? 1 : -1;
  return [...accounts].sort((a, b) => {
    let av: string;
    let bv: string;
    switch (sort.key) {
      case "name":
        av = getAccountName(a);
        bv = getAccountName(b);
        break;
      case "email":
        av = a.email.toLowerCase();
        bv = b.email.toLowerCase();
        break;
      case "roleName":
        av = a.roleName.toLowerCase();
        bv = b.roleName.toLowerCase();
        break;
      case "pedagogicRole":
        av = a.pedagogicRole.toLowerCase();
        bv = b.pedagogicRole.toLowerCase();
        break;
      case "status":
        av = a.status.toLowerCase();
        bv = b.status.toLowerCase();
        break;
    }
    return av < bv ? -dir : av > bv ? dir : 0;
  });
}

function AccountsTable({ accounts }: { readonly accounts: SchoolAccount[] }) {
  const { sort, toggle } = useSort<SchoolAccountSortKey>("name");
  const sorted = useMemo(
    () => sortSchoolAccounts(accounts, sort),
    [accounts, sort],
  );

  return (
    <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
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
            <SortableHeader
              label="Status"
              sortKey="status"
              sort={sort}
              onToggle={toggle}
            />
          </tr>
        </thead>
        <tbody>
          {sorted.map((account) => (
            <tr
              key={
                account.accountId !== "0"
                  ? account.accountId
                  : `invited-${account.email}`
              }
              className="border-b border-gray-50 last:border-0"
            >
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
                <AccountStatusBadge status={account.status} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

type OrgAccountSortKey = "schoolName" | SchoolAccountSortKey;

function sortOrgAccounts(
  accounts: readonly OrgAccount[],
  sort: SortState<OrgAccountSortKey>,
): OrgAccount[] {
  const dir = sort.direction === "asc" ? 1 : -1;
  return [...accounts].sort((a, b) => {
    let av: string;
    let bv: string;
    switch (sort.key) {
      case "schoolName":
        av = a.schoolName.toLowerCase();
        bv = b.schoolName.toLowerCase();
        break;
      case "name":
        av = getAccountName(a);
        bv = getAccountName(b);
        break;
      case "email":
        av = a.email.toLowerCase();
        bv = b.email.toLowerCase();
        break;
      case "roleName":
        av = a.roleName.toLowerCase();
        bv = b.roleName.toLowerCase();
        break;
      case "pedagogicRole":
        av = a.pedagogicRole.toLowerCase();
        bv = b.pedagogicRole.toLowerCase();
        break;
      case "status":
        av = a.status.toLowerCase();
        bv = b.status.toLowerCase();
        break;
    }
    return av < bv ? -dir : av > bv ? dir : 0;
  });
}

function OrgAccountsTable({ accounts }: { readonly accounts: OrgAccount[] }) {
  const { sort, toggle } = useSort<OrgAccountSortKey>("schoolName");
  const sorted = useMemo(
    () => sortOrgAccounts(accounts, sort),
    [accounts, sort],
  );

  return (
    <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
            <SortableHeader
              label="Schule"
              sortKey="schoolName"
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
            <SortableHeader
              label="Status"
              sortKey="status"
              sort={sort}
              onToggle={toggle}
            />
          </tr>
        </thead>
        <tbody>
          {sorted.map((account) => (
            <tr
              key={
                account.accountId !== "0"
                  ? `${account.schoolId}-${account.accountId}`
                  : `${account.schoolId}-invited-${account.email}`
              }
              className="border-b border-gray-50 last:border-0"
            >
              <td className="px-5 py-3 text-gray-600">{account.schoolName}</td>
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
                <AccountStatusBadge status={account.status} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
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
}: {
  readonly devices: OperatorDevice[];
  readonly showSchool?: boolean;
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
