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
      className: "bg-moto-green/15 text-moto-green-strong",
    };
  }
  if (account.isActiveCaregiver) {
    return {
      label: "Betreuung aktiv",
      className: "bg-moto-orange/15 text-moto-orange-strong",
    };
  }
  if (account.hasUserRole && !account.hasCaregiverProfile) {
    return {
      label: "Betreuung unvollständig",
      className: "bg-moto-amber/15 text-moto-amber-strong",
    };
  }
  if (account.hasCaregiverProfile && !account.hasUserRole) {
    return {
      label: "Betreuungsprofil inaktiv",
      className: "bg-gray-100 text-gray-700",
    };
  }
  if (account.hasAdminRole) {
    return {
      label: "Nur Verwaltung",
      className: "bg-moto-blue/15 text-moto-blue-hover",
    };
  }
  return {
    label: "Keine Betreuung",
    className: "bg-gray-100 text-gray-600",
  };
}

function AccountStatusBadge({ status }: Readonly<{ status: string }>) {
  const styles: Record<string, string> = {
    active: "bg-moto-green/15 text-moto-green-strong",
    pending: "bg-moto-amber/15 text-moto-amber-strong",
    invited: "bg-moto-purple/15 text-moto-purple-strong",
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
  isLoading?: boolean;
  selectedSchool?: School | null;
  onManageCaregiver?: (
    account: AccountRow,
    schoolContext: { id: string; name: string } | null,
  ) => void;
  onManageMFA?: (
    account: AccountRow,
    schoolContext: { id: string; name: string } | null,
  ) => void;
  // Schulzugänge are account-scoped, not school-scoped — no school context.
  onManageTenantAccess?: (account: AccountRow) => void;
}

export function AccountsTable({
  accounts,
  showSchool = false,
  isLoading = false,
  selectedSchool,
  onManageCaregiver,
  onManageMFA,
  onManageTenantAccess,
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
                  className="bg-moto-blue/10 text-moto-blue-hover inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
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
          const isActionable =
            row.accountId !== "0" &&
            row.status !== "invited" &&
            schoolContext != null;
          const canManageCaregiver = isActionable && onManageCaregiver != null;
          const canManageMFA = isActionable && onManageMFA != null;
          // Managing school access needs no school context, so it stays
          // available even when the row has none.
          const canManageTenantAccess =
            row.accountId !== "0" &&
            row.status !== "invited" &&
            onManageTenantAccess != null;
          if (!canManageCaregiver && !canManageMFA && !canManageTenantAccess) {
            return <span className="text-xs text-gray-400">—</span>;
          }
          return (
            <div className="flex flex-wrap justify-end gap-2">
              {canManageCaregiver && (
                <button
                  type="button"
                  onClick={() => onManageCaregiver?.(row, schoolContext)}
                  className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50"
                >
                  Betreuung verwalten
                </button>
              )}
              {canManageMFA && (
                <button
                  type="button"
                  onClick={() => onManageMFA?.(row, schoolContext)}
                  className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50"
                >
                  2FA verwalten
                </button>
              )}
              {canManageTenantAccess && (
                <button
                  type="button"
                  onClick={() => onManageTenantAccess?.(row)}
                  className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50"
                >
                  Schulzugänge
                </button>
              )}
            </div>
          );
        },
      },
    );

    return cols;
  }, [
    showSchool,
    selectedSchool,
    onManageCaregiver,
    onManageMFA,
    onManageTenantAccess,
  ]);

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
      pageSize={50}
      isLoading={isLoading}
    />
  );
}
