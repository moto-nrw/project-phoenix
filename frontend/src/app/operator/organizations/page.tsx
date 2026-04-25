"use client";

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
import type { Organization } from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";
import {
  StatusBadge,
  EmptyState,
  PlusIcon,
  CardSkeletons,
} from "../provisioning/provisioning-shared";
import { CreateOrganizationModal } from "../provisioning/create-organization-modal";
import { EditOrganizationModal } from "../provisioning/edit-organization-modal";
import {
  useSoftDeletable,
  DeletedEntityCard,
  SoftDeleteConfirmationModal,
  RestoreConfirmationModal,
} from "../provisioning/soft-delete-shared";

const logger = createLogger({ component: "OperatorOrganizationsPage" });

export default function OperatorOrganizationsPage() {
  const { status } = useSession();
  const isAuthenticated = status === "authenticated";
  useSetBreadcrumb({ pageTitle: "Träger" });

  const [createOrgOpen, setCreateOrgOpen] = useState(false);
  const [editOrgOpen, setEditOrgOpen] = useState(false);
  const [editOrgTarget, setEditOrgTarget] = useState<Organization | null>(null);
  const [orgToggleError, setOrgToggleError] = useState("");

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

  // Schools are needed to cascade tenant cache revalidation when toggling
  // an organization's active state, and to block deletion when an org still
  // owns active schools.
  const { data: schools, mutate: mutateSchools } = useSWR(
    isAuthenticated ? "operator-schools" : null,
    () => operatorProvisioningService.listSchools(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const openEditOrg = useCallback((org: Organization) => {
    setEditOrgTarget(org);
    setEditOrgOpen(true);
  }, []);

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
        await revalidateTenantCache(orgSchoolSlugs);
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

  const activeOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt == null) ?? [],
    [organizations],
  );

  const deletedOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt != null) ?? [],
    [organizations],
  );

  const activeSchools = useMemo(
    () => schools?.filter((s) => s.deletedAt == null) ?? [],
    [schools],
  );

  const orgDelete = useSoftDeletable<Organization>({
    softDeleteFn: operatorProvisioningService.softDeleteOrganization,
    restoreFn: operatorProvisioningService.restoreOrganization,
    mutateList: mutateOrgs,
    errorMessages: {
      softDelete:
        "Fehler beim Löschen des Trägers. Bitte versuchen Sie es erneut.",
      restore:
        "Fehler beim Wiederherstellen des Trägers. Bitte versuchen Sie es erneut.",
    },
    logEventPrefix: "organization",
  });

  const orgDeleteTargetHasSchools = useMemo(() => {
    const orgId = orgDelete.deleteTarget?.id;
    if (!orgId) return false;
    return activeSchools.some((s) => s.organizationId === orgId);
  }, [orgDelete.deleteTarget, activeSchools]);

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "organizations",
          label: "Träger",
          count: organizations?.length,
        },
      ],
      activeTab: "organizations",
      onTabChange: () => undefined,
    }),
    [organizations?.length],
  );

  const actionButton = (
    <button
      type="button"
      onClick={() => setCreateOrgOpen(true)}
      className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
    >
      Neuer Träger
    </button>
  );

  const mobileActionButton = (
    <button
      type="button"
      onClick={() => setCreateOrgOpen(true)}
      className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
      aria-label="Neuer Träger"
    >
      <PlusIcon />
    </button>
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Träger"
        tabs={tabs}
        actionButton={actionButton}
        mobileActionButton={mobileActionButton}
      />

      {orgsLoading && <CardSkeletons />}

      {!orgsLoading && (
        <>
          {deletedOrganizations.length > 0 && (
            <div className="mb-4 flex justify-end">
              <button
                type="button"
                onClick={() => orgDelete.setShowTrash(!orgDelete.showTrash)}
                className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                  orgDelete.showTrash
                    ? "bg-red-100 text-red-700 hover:bg-red-200"
                    : "bg-gray-100 text-gray-700 hover:bg-gray-200"
                }`}
              >
                Papierkorb ({deletedOrganizations.length})
              </button>
            </div>
          )}

          {orgDelete.showTrash ? (
            <div className="mt-4 space-y-4">
              {deletedOrganizations.map((org) => (
                <DeletedEntityCard
                  key={org.id}
                  name={org.name}
                  subtitle={org.slug}
                  deletedAt={org.deletedAt}
                  onRestore={() => orgDelete.setRestoreTarget(org)}
                />
              ))}
            </div>
          ) : (
            <>
              {activeOrganizations.length === 0 && (
                <EmptyState
                  title="Keine Träger"
                  description="Erstellen Sie einen neuen Träger, um Schulen zu verwalten."
                  buttonLabel="Neuer Träger"
                  onAction={() => setCreateOrgOpen(true)}
                />
              )}
              {activeOrganizations.length > 0 && (
                <div className="mt-4 space-y-4">
                  {activeOrganizations.map((org) => (
                    <OrganizationCard
                      key={org.id}
                      organization={org}
                      onEdit={openEditOrg}
                      onToggleActive={handleToggleOrgActive}
                      onDelete={orgDelete.setDeleteTarget}
                    />
                  ))}
                </div>
              )}
            </>
          )}
          {orgToggleError && (
            <p className="mt-2 text-sm text-red-600">{orgToggleError}</p>
          )}
        </>
      )}

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
          inputId="delete-org-confirm"
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
        target={orgDelete.restoreTarget}
        setTarget={orgDelete.setRestoreTarget}
        entityLabel="Träger"
        entityArticleAccusative="den Träger"
        entityPronounNominative="Der Träger"
        entityPossessiveAccusative="seinen"
        extraMessage="Gelöschte Schulen dieses Trägers müssen separat wiederhergestellt werden."
        onConfirm={() => void orgDelete.handleRestore()}
        isProcessing={orgDelete.isProcessing}
        errorMessage={orgDelete.softDeleteError}
      />

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
        onUpdated={async () => {
          await Promise.all([mutateOrgs(), mutateSchools()]);
        }}
      />
    </div>
  );
}

function OrganizationCard({
  organization,
  onEdit,
  onToggleActive,
  onDelete,
}: {
  readonly organization: Organization;
  readonly onEdit: (org: Organization) => void;
  readonly onToggleActive: (org: Organization) => Promise<void>;
  readonly onDelete: (org: Organization) => void;
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
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => onEdit(organization)}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Bearbeiten
          </button>
          <button
            type="button"
            onClick={() => onDelete(organization)}
            className="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-100"
          >
            Löschen
          </button>
        </div>
      </div>
    </div>
  );
}
