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
  OperatorPerson,
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
import { DataTableStatusBadge } from "~/components/ui/data-table";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { CaregiverCapabilityModal } from "~/components/teachers";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { createLogger } from "~/lib/logger";
import { PlusIcon } from "~/app/operator/provisioning/provisioning-shared";
import { EditSchoolModal } from "~/app/operator/provisioning/edit-school-modal";
import { InviteAdminModal } from "~/app/operator/provisioning/invite-admin-modal";
import { CreateAccountModal } from "~/app/operator/provisioning/create-account-modal";
import { CreateDeviceModal } from "~/app/operator/provisioning/create-device-modal";
import { SetApiKeyModal } from "~/app/operator/provisioning/set-api-key-modal";
import {
  SoftDeleteConfirmationModal,
  useSoftDeletable,
} from "~/app/operator/provisioning/soft-delete-shared";

const logger = createLogger({ component: "OperatorSchoolDetailPage" });

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
  const { slug, schoolSlug } = use(params);
  const { status } = useSession();
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
  const [editSchoolOpen, setEditSchoolOpen] = useState(false);
  const [schoolToggleError, setSchoolToggleError] = useState("");
  const [inviteOpen, setInviteOpen] = useState(false);
  const [createAccountOpen, setCreateAccountOpen] = useState(false);
  const [createDeviceOpen, setCreateDeviceOpen] = useState(false);
  const [setKeyDevice, setSetKeyDevice] = useState<OperatorDevice | null>(null);
  const [deleteDevice, setDeleteDeviceRaw] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteConfirmed, setDeleteConfirmed] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const [deletePersonTarget, setDeletePersonTargetRaw] =
    useState<OperatorPerson | null>(null);
  const [deletePersonConfirmInput, setDeletePersonConfirmInput] = useState("");
  const [deletePersonLoading, setDeletePersonLoading] = useState(false);
  const [deletePersonError, setDeletePersonError] = useState("");

  const currentTabSearch = activeTab === "konten" ? "" : `?tab=${activeTab}`;

  const setDeleteDevice = useCallback((device: OperatorDevice | null) => {
    setDeleteDeviceRaw(device);
    setDeleteConfirmed(false);
    setDeleteError("");
  }, []);

  const setDeletePersonTarget = useCallback((person: OperatorPerson | null) => {
    setDeletePersonTargetRaw(person);
    setDeletePersonConfirmInput("");
    setDeletePersonError("");
  }, []);

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

  const selectedSchoolForTable = useMemo(
    () => (school ? summaryToSchool(school) : null),
    [school],
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

  const handleDeleteDevice = useCallback(async () => {
    if (!deleteDevice) return;
    setDeleteLoading(true);
    setDeleteError("");
    try {
      await operatorProvisioningService.deleteDevice(deleteDevice.id);
      setDeleteDevice(null);
      setDeleteConfirmed(false);
      await Promise.all([mutateSchoolDevices(), refreshSchoolDetail()]);
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
  }, [deleteDevice, mutateSchoolDevices, refreshSchoolDetail, setDeleteDevice]);

  const handleDeletePerson = useCallback(async () => {
    if (!deletePersonTarget) return;
    setDeletePersonLoading(true);
    setDeletePersonError("");
    try {
      await operatorProvisioningService.softDeletePerson(deletePersonTarget.id);
      setDeletePersonTarget(null);
      await Promise.all([mutateSchoolPersons(), refreshSchoolDetail()]);
    } catch (error) {
      logger.error("person_soft_delete_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setDeletePersonError(
        error instanceof Error
          ? error.message
          : "Fehler beim Löschen der Person",
      );
    } finally {
      setDeletePersonLoading(false);
    }
  }, [
    deletePersonTarget,
    mutateSchoolPersons,
    refreshSchoolDetail,
    setDeletePersonTarget,
  ]);

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
          href={school ? `/operator/schools/${school.id}/settings` : "#"}
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
          className="rounded-lg bg-red-50 px-3 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-100"
        >
          Löschen
        </button>
      </>
    ),
    [handleToggleSchoolActive, school, schoolDelete],
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
              <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-700">
                Verborgen
              </span>
            )}
          </span>
        }
        stats={[
          { label: "Konten", value: numberFormat(school.kontenCount) },
          { label: "Geräte", value: numberFormat(school.geraeteCount) },
          { label: "Personen", value: numberFormat(school.personenCount) },
        ]}
      />

      {schoolToggleError && (
        <p className="mt-3 text-sm text-red-600">{schoolToggleError}</p>
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

          <TabsPrimitive.Content value="konten" className="mt-4">
            {accountsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : schoolAccounts && schoolAccounts.length > 0 ? (
              <AccountsTable
                accounts={schoolAccounts}
                selectedSchool={selectedSchoolForTable}
                onManageCaregiver={openCaregiverModal}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Konten für diese Schule.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="geraete" className="mt-4">
            {devicesLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : schoolDevices && schoolDevices.length > 0 ? (
              <DevicesTable
                devices={schoolDevices}
                onSetKey={setSetKeyDevice}
                onDelete={setDeleteDevice}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Geräte für diese Schule.
              </div>
            )}
          </TabsPrimitive.Content>

          <TabsPrimitive.Content value="personen" className="mt-4">
            {personsLoading ? (
              <div className="py-10 text-center text-gray-500">
                Wird geladen…
              </div>
            ) : schoolPersons && schoolPersons.length > 0 ? (
              <PersonsTable
                persons={schoolPersons}
                onDelete={setDeletePersonTarget}
              />
            ) : (
              <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 px-6 py-10 text-center text-sm text-gray-500">
                Keine Personen für diese Schule.
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
            await mutateSchoolAccounts();
          }}
        />
      ) : null}

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
                htmlFor="delete-person-confirm-detail"
                className="block text-sm font-medium text-gray-700"
              >
                Geben Sie den vollständigen Namen ein:
              </label>
              <p className="mb-1 text-sm font-medium text-gray-900">
                {deletePersonTarget.fullName}
              </p>
              <input
                id="delete-person-confirm-detail"
                type="text"
                value={deletePersonConfirmInput}
                onChange={(event) =>
                  setDeletePersonConfirmInput(event.target.value)
                }
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
    </div>
  );
}
