"use client";

import { Button } from "~/components/ui/button";
import { useMemo } from "react";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { StatusBadge } from "~/components/ui/status-badge";

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
   * Zeigt einen Strich („–“), wenn die Person kein einziges Merkmal traegt.
   * Nur fuer die Tabellenzelle gedacht, die nicht leer bleiben soll. Die
   * Kartenansicht rendert in dem Fall nichts, so wie vor der Extraktion.
   */
  emptyPlaceholder?: boolean;
}>) {
  if (!person.isStaff && !person.isStudent && !person.hasRfidCard) {
    return emptyPlaceholder ? (
      <span className="text-xs text-gray-400">–</span>
    ) : null;
  }
  return (
    <div className="flex flex-wrap gap-1">
      {person.isStaff && (
        <StatusBadge tone="blue" compact label="Mitarbeiter" />
      )}
      {person.isStudent && <StatusBadge tone="green" compact label="Kinder" />}
      {person.hasRfidCard && <StatusBadge tone="gray" compact label="RFID" />}
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
            <span className="text-xs text-gray-400">–</span>
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
            <Button
              type="button"
              variant="ghost_danger"
              size="compact"
              onClick={() => onDelete(row)}
              title="Person löschen"
            >
              Löschen
            </Button>
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
