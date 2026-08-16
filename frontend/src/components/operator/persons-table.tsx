"use client";

import { useMemo } from "react";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";

interface PersonsTableProps {
  persons: OperatorPerson[];
  showSchool?: boolean;
  isLoading?: boolean;
  onDelete?: (person: OperatorPerson) => void;
}

/**
 * Merkmals-Badges (Mitarbeiter/Kinder/RFID) fuer eine Person. Gemeinsame
 * Implementierung fuer PersonsTable und die Karten-Ansicht in
 * app/operator/persons/page.tsx, damit beide dieselben MOTO-Tokens nutzen.
 */
export function PersonTags({
  person,
  emptyPlaceholder = false,
}: Readonly<{
  person: OperatorPerson;
  /**
   * Zeigt einen Gedankenstrich, wenn die Person kein einziges Merkmal traegt.
   * Nur fuer die Tabellenzelle gedacht, die nicht leer bleiben soll. Die
   * Kartenansicht rendert in dem Fall nichts, so wie vor der Extraktion.
   */
  emptyPlaceholder?: boolean;
}>) {
  if (!person.isStaff && !person.isStudent && !person.hasRfidCard) {
    return emptyPlaceholder ? (
      <span className="text-xs text-gray-400">—</span>
    ) : null;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {person.isStaff && (
        <span className="bg-moto-blue/10 text-moto-blue-hover inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium">
          Mitarbeiter
        </span>
      )}
      {person.isStudent && (
        <span className="bg-moto-green/10 text-moto-green-strong inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium">
          Kinder
        </span>
      )}
      {person.hasRfidCard && (
        <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
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
  isLoading = false,
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
        render: (row) => <PersonTags person={row} emptyPlaceholder />,
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
              className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red/10 hover:text-moto-red rounded-lg border px-2 py-1 text-xs font-medium transition-colors"
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
      pageSize={50}
      isLoading={isLoading}
    />
  );
}
