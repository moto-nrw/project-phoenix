"use client";

import { useMemo } from "react";
import type {
  OrgAccount,
  School,
  SchoolAccount,
} from "~/lib/operator/provisioning-helpers";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";

type AccountRow = SchoolAccount | OrgAccount;

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

function getDisplayName(account: AccountRow): string {
  return account.firstName || account.lastName
    ? `${account.firstName} ${account.lastName}`.trim()
    : "—";
}

function getNameSortValue(account: AccountRow): string {
  return account.firstName || account.lastName
    ? `${account.lastName} ${account.firstName}`.trim().toLowerCase()
    : "";
}

interface AccountsTableProps {
  accounts: SchoolAccount[] | OrgAccount[];
  showSchool?: boolean;
  selectedSchool?: School | null;
  onManageCaregiver?: (
    account: AccountRow,
    schoolContext: { id: string; name: string } | null,
  ) => void;
}

export function AccountsTable({
  accounts,
  showSchool = false,
  selectedSchool,
  onManageCaregiver,
}: Readonly<AccountsTableProps>) {
  const columns = useMemo<DataTableColumn<AccountRow>[]>(() => {
    const cols: DataTableColumn<AccountRow>[] = [];

    if (showSchool) {
      cols.push({
        key: "schoolName",
        header: "Schule",
        render: (row) => (row as OrgAccount).schoolName,
        sortValue: (row) => (row as OrgAccount).schoolName.toLowerCase(),
        className: "text-gray-600",
      });
    }

    cols.push(
      {
        key: "name",
        header: "Name",
        render: getDisplayName,
        sortValue: getNameSortValue,
        className: "font-medium text-gray-900",
      },
      {
        key: "email",
        header: "E-Mail",
        render: (row) => row.email,
        sortValue: (row) => row.email.toLowerCase(),
        className: "text-gray-600",
      },
      {
        key: "roleName",
        header: "Rolle",
        render: (row) =>
          row.roleName ? (
            <span className="inline-flex flex-wrap gap-1">
              {row.roleName.split(", ").map((role) => (
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
          ),
        sortValue: (row) => row.roleName.toLowerCase(),
      },
      {
        key: "pedagogicRole",
        header: "Päd. Rolle",
        render: (row) => row.pedagogicRole || "—",
        sortValue: (row) => row.pedagogicRole.toLowerCase(),
        className: "text-gray-600",
      },
      {
        key: "capability",
        header: "Einsatz",
        render: (row) => {
          const capability = getAccountCapabilityMeta(row);
          return (
            <span
              className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${capability.className}`}
            >
              {capability.label}
            </span>
          );
        },
      },
      {
        key: "status",
        header: "Status",
        render: (row) => <AccountStatusBadge status={row.status} />,
        sortValue: (row) => row.status.toLowerCase(),
      },
      {
        key: "action",
        header: "Aktion",
        align: "right",
        render: (row) => {
          const orgAccount = showSchool ? (row as OrgAccount) : undefined;
          const schoolContext = showSchool
            ? orgAccount
              ? { id: orgAccount.schoolId, name: orgAccount.schoolName }
              : null
            : selectedSchool
              ? { id: selectedSchool.id, name: selectedSchool.name }
              : null;
          const canManageCaregiver =
            row.accountId !== "0" &&
            row.status !== "invited" &&
            onManageCaregiver != null &&
            schoolContext != null;
          if (!canManageCaregiver) {
            return <span className="text-xs text-gray-400">—</span>;
          }
          return (
            <button
              type="button"
              onClick={() => onManageCaregiver?.(row, schoolContext)}
              className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Betreuung verwalten
            </button>
          );
        },
      },
    );

    return cols;
  }, [showSchool, selectedSchool, onManageCaregiver]);

  return (
    <DataTable
      columns={columns}
      rows={accounts as AccountRow[]}
      getRowKey={(row) => {
        const orgAccount = showSchool ? (row as OrgAccount) : undefined;
        return row.accountId !== "0"
          ? `${orgAccount?.schoolId ?? ""}-${row.accountId}`
          : `${orgAccount?.schoolId ?? ""}-invited-${row.email}`;
      }}
      defaultSortKey={showSchool ? "schoolName" : "name"}
    />
  );
}
