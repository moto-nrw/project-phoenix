"use client";

import { useMemo } from "react";
import type {
  OrgAccount,
  School,
  SchoolAccount,
} from "~/lib/operator/provisioning-helpers";
import {
  SortableHeader,
  useSort,
  type SortState,
} from "~/app/operator/provisioning/provisioning-tables-shared";

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

function AccountStatusBadge({ status }: Readonly<{ status: string }>) {
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

function AccountRowWithAction({
  account,
  schoolContext,
  onManageCaregiver,
}: Readonly<{
  account: SchoolAccount | OrgAccount;
  schoolContext: { id: string; name: string } | null;
  onManageCaregiver?: (
    account: SchoolAccount | OrgAccount,
    schoolContext: { id: string; name: string } | null,
  ) => void;
}>) {
  const capability = getAccountCapabilityMeta(account);
  const canManageCaregiver =
    account.accountId !== "0" &&
    account.status !== "invited" &&
    onManageCaregiver != null &&
    schoolContext != null;

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

interface AccountsTableProps {
  accounts: SchoolAccount[] | OrgAccount[];
  showSchool?: boolean;
  selectedSchool?: School | null;
  onManageCaregiver?: (
    account: SchoolAccount | OrgAccount,
    schoolContext: { id: string; name: string } | null,
  ) => void;
}

export function AccountsTable({
  accounts,
  showSchool = false,
  selectedSchool,
  onManageCaregiver,
}: Readonly<AccountsTableProps>) {
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
