"use client";

import { useMemo } from "react";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";

interface PersonsTableProps {
  persons: OperatorPerson[];
  showSchool?: boolean;
  onDelete?: (person: OperatorPerson) => void;
}

function PersonTags({ person }: Readonly<{ person: OperatorPerson }>) {
  if (!person.isStaff && !person.isStudent && !person.hasRfidCard) {
    return <span className="text-xs text-gray-400">—</span>;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {person.isStaff && (
        <span className="inline-flex items-center rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-medium text-[#4070C8]">
          Mitarbeiter
        </span>
      )}
      {person.isStudent && (
        <span className="inline-flex items-center rounded-full bg-[#83CD2D]/10 px-2 py-0.5 text-xs font-medium text-[#5A8B1F]">
          Schüler
        </span>
      )}
      {person.hasRfidCard && (
        <span className="inline-flex items-center rounded-full bg-[#7C3AED]/10 px-2 py-0.5 text-xs font-medium text-[#5B21B6]">
          RFID
        </span>
      )}
    </div>
  );
}

function getPersonTagSortValue(person: OperatorPerson): string {
  const tags: string[] = [];
  if (person.isStaff) tags.push("mitarbeiter");
  if (person.isStudent) tags.push("schüler");
  if (person.hasRfidCard) tags.push("rfid");
  return tags.join(",");
}

export function PersonsTable({
  persons,
  showSchool = false,
  onDelete,
}: Readonly<PersonsTableProps>) {
  const columns: DataTableColumn<OperatorPerson>[] = useMemo(() => {
    const cols: DataTableColumn<OperatorPerson>[] = [
      {
        key: "name",
        header: "Name",
        render: (row) => (
          <span className="font-medium text-gray-900">
            {row.firstName} {row.lastName}
          </span>
        ),
        sortValue: (row) =>
          `${row.lastName} ${row.firstName}`.trim().toLowerCase(),
      },
    ];
    if (showSchool) {
      cols.push({
        key: "school",
        header: "Schule",
        render: (row) => (
          <span className="text-gray-700">{row.schoolName}</span>
        ),
        sortValue: (row) => row.schoolName.toLowerCase(),
      });
    }
    cols.push(
      {
        key: "tags",
        header: "Merkmale",
        render: (row) => <PersonTags person={row} />,
        sortValue: getPersonTagSortValue,
      },
      {
        key: "email",
        header: "E-Mail",
        render: (row) =>
          row.accountEmail ? (
            <span className="text-gray-600">{row.accountEmail}</span>
          ) : (
            <span className="text-xs text-gray-400">—</span>
          ),
        sortValue: (row) => (row.accountEmail ?? "").toLowerCase(),
      },
    );
    if (onDelete) {
      cols.push({
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (row) => (
          <div
            className="flex justify-end"
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
            role="presentation"
          >
            <button
              type="button"
              onClick={() => onDelete(row)}
              className="rounded-lg border border-[#FF3130]/20 px-2 py-1 text-xs font-medium text-[#CC2626] transition-colors hover:bg-[#FF3130]/10 hover:text-[#FF3130]"
              title="Person löschen"
            >
              Löschen
            </button>
          </div>
        ),
      });
    }
    return cols;
  }, [onDelete, showSchool]);

  return (
    <DataTable
      columns={columns}
      rows={persons}
      getRowKey={(row) => row.id}
      emptyState="Keine Personen vorhanden."
      defaultSortKey="name"
    />
  );
}
