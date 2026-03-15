"use client";

import { useState, useMemo, useCallback } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages use useOperatorAuth, not NextAuth
import useSWR from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { useOperatorAuth } from "~/lib/operator/auth-context";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
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

type ActiveTab = "organizations" | "schools";

export default function OperatorProvisioningPage() {
  const { isAuthenticated } = useOperatorAuth();
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

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "organizations",
          label: "Träger",
          count: organizations?.length,
        },
        { id: "schools", label: "Schulen", count: schools?.length },
      ],
      activeTab,
      onTabChange: (tabId: string) => setActiveTab(tabId as ActiveTab),
    }),
    [activeTab, organizations?.length, schools?.length],
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
    activeTab === "organizations" ? orgsLoading : schoolsLoading;

  const actionButton =
    activeTab === "organizations" ? (
      <button
        type="button"
        onClick={() => setCreateOrgOpen(true)}
        className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        Neuer Träger
      </button>
    ) : (
      <button
        type="button"
        onClick={() => setCreateSchoolOpen(true)}
        className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        Neue Schule
      </button>
    );

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
    ) : (
      <button
        type="button"
        onClick={() => setCreateSchoolOpen(true)}
        className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
        aria-label="Neue Schule"
      >
        <PlusIcon />
      </button>
    );

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
                />
              ))}
            </div>
          )}
          {schoolToggleError && (
            <p className="mt-2 text-sm text-red-600">{schoolToggleError}</p>
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
}: {
  readonly school: School;
  readonly onEdit: (school: School) => void;
  readonly onToggleActive: (school: School) => Promise<void>;
  readonly onInviteAdmin: (school: School) => void;
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
