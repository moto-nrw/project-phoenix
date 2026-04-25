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
import type { School } from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";
import { operatorPath } from "~/lib/operator-url";
import {
  StatusBadge,
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

  const activeOrganizations = useMemo(
    () => organizations?.filter((o) => o.deletedAt == null) ?? [],
    [organizations],
  );

  const activeSchools = useMemo(
    () => schools?.filter((s) => s.deletedAt == null) ?? [],
    [schools],
  );

  const deletedSchools = useMemo(
    () => schools?.filter((s) => s.deletedAt != null) ?? [],
    [schools],
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

  const refreshAccounts = useCallback(() => {
    return globalMutate(
      (key: unknown) =>
        typeof key === "string" &&
        ACCOUNT_SWR_PREFIXES.some((p) => key.startsWith(p)),
    );
  }, [globalMutate]);

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

  const openSchoolAccounts = useCallback(
    (school: School) => {
      const target = operatorPath(
        `/operator/accounts?orgId=${encodeURIComponent(school.organizationId)}&schoolId=${encodeURIComponent(school.id)}`,
      );
      router.push(target);
    },
    [router],
  );

  const handleToggleSchoolActive = useCallback(
    async (school: School) => {
      setSchoolToggleError("");
      try {
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
        await revalidateTenantCache([fresh.subdomain]);
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

  const schoolDelete = useSoftDeletable<School>({
    softDeleteFn: operatorProvisioningService.softDeleteSchool,
    restoreFn: operatorProvisioningService.restoreSchool,
    mutateList: mutateSchools,
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

  const schoolRestoreParentDeleted = useMemo(() => {
    const target = schoolDelete.restoreTarget;
    if (!target) return false;
    if (target.organization?.deletedAt != null) return true;
    return deletedOrgIds.has(target.organizationId);
  }, [schoolDelete.restoreTarget, deletedOrgIds]);

  const tabs = useMemo(
    () => ({
      items: [{ id: "schools", label: "Schulen", count: schools?.length }],
      activeTab: "schools",
      onTabChange: () => undefined,
    }),
    [schools?.length],
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

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Schulen"
        tabs={tabs}
        actionButton={actionButton}
        mobileActionButton={mobileActionButton}
      />

      {schoolsLoading && <CardSkeletons />}

      {!schoolsLoading && (
        <>
          {deletedSchools.length > 0 && (
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
                Papierkorb ({deletedSchools.length})
              </button>
            </div>
          )}

          {schoolDelete.showTrash ? (
            <div className="mt-4 space-y-4">
              {deletedSchools.map((school) => {
                const parentOrgDeleted =
                  school.organization?.deletedAt != null ||
                  deletedOrgIds.has(school.organizationId);
                return (
                  <DeletedEntityCard
                    key={school.id}
                    name={school.name}
                    subtitle={school.subdomain}
                    extraSubtitle={school.organization?.name}
                    deletedAt={school.deletedAt}
                    onRestore={() => schoolDelete.setRestoreTarget(school)}
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
                      onDelete={schoolDelete.setDeleteTarget}
                    />
                  ))}
                </div>
              )}
            </>
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
        </>
      )}

      <CreateSchoolModal
        isOpen={createSchoolOpen}
        onClose={() => setCreateSchoolOpen(false)}
        organizations={activeOrganizations}
        onCreated={() => mutateSchools().then(() => undefined)}
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
          await Promise.all([mutateSchools(), mutateOrgs()]);
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
          <Link
            href={`/operator/schools/${school.id}/settings`}
            className="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200"
          >
            Einstellungen
          </Link>
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
