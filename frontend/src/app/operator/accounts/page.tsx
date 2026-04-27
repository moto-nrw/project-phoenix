"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR, { useSWRConfig } from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  OrgAccount,
  School,
  SchoolAccount,
} from "~/lib/operator/provisioning-helpers";
import {
  CardSkeletons,
  SimpleEmptyState,
} from "../provisioning/provisioning-shared";
import { CaregiverCapabilityModal } from "~/components/teachers";
import {
  OrgSchoolFilter,
  SortableHeader,
  useSort,
  type SortState,
} from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";

const ACCOUNT_SWR_PREFIXES = [
  "operator-all-accounts",
  "operator-school-accounts-",
  "operator-org-accounts-",
];

function OperatorAccountsPageContent() {
  useSetBreadcrumb({ pageTitle: "Konten" });

  const {
    isAuthenticated,
    activeOrganizations,
    filterOrgId,
    selectedSchool,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
  } = useOrgSchoolFilter("/operator/accounts");

  const [caregiverAccount, setCaregiverAccount] = useState<
    SchoolAccount | OrgAccount | null
  >(null);
  const [caregiverSchoolContext, setCaregiverSchoolContext] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const { mutate: globalMutate } = useSWRConfig();

  const { data: schoolAccounts, isLoading: schoolAccountsLoading } = useSWR(
    isAuthenticated && selectedSchool
      ? `operator-school-accounts-${selectedSchool.id}`
      : null,
    () => operatorProvisioningService.listSchoolAccounts(selectedSchool!.id),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: orgAccounts, isLoading: orgAccountsLoading } = useSWR(
    isAuthenticated && filterOrgId && !selectedSchool
      ? `operator-org-accounts-${filterOrgId}`
      : null,
    () => operatorProvisioningService.listOrganizationAccounts(filterOrgId),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: allAccounts, isLoading: allAccountsLoading } = useSWR(
    isAuthenticated && !filterOrgId && !selectedSchool
      ? "operator-all-accounts"
      : null,
    () => operatorProvisioningService.listAllAccounts(),
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

  const refreshAccounts = useCallback(() => {
    return globalMutate(
      (key: unknown) =>
        typeof key === "string" &&
        ACCOUNT_SWR_PREFIXES.some((p) => key.startsWith(p)),
    );
  }, [globalMutate]);

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

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "accounts",
          label: "Konten",
          count: selectedSchool
            ? schoolAccounts?.length
            : filterOrgId
              ? orgAccounts?.length
              : allAccounts?.length,
        },
      ],
      activeTab: "accounts",
      onTabChange: () => undefined,
    }),
    [
      selectedSchool,
      filterOrgId,
      schoolAccounts?.length,
      orgAccounts?.length,
      allAccounts?.length,
    ],
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Konten" tabs={tabs} />

      <OrgSchoolFilter
        idPrefix="account"
        organizations={activeOrganizations}
        filteredSchools={filteredSchools}
        filterOrgId={filterOrgId}
        selectedSchool={selectedSchool}
        onOrgChange={handleOrgFilterChange}
        onSchoolChange={handleSchoolFilterChange}
      />

      {accountsLoading && <CardSkeletons />}

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
    </div>
  );
}

export default function OperatorAccountsPage() {
  return (
    <Suspense fallback={<div className="-mt-1.5 w-full" />}>
      <OperatorAccountsPageContent />
    </Suspense>
  );
}

// --- Account table helpers (tab-local) ---

type AccountSortKey =
  | "name"
  | "email"
  | "roleName"
  | "pedagogicRole"
  | "status"
  | "schoolName";

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
