import { DataTableStatusBadge } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import { formatCount } from "~/lib/format-utils";
import type { SchoolSummary } from "~/lib/operator/provisioning-helpers";

interface BuildSchoolColumnsOptions {
  readonly showOrgColumn?: boolean;
}

export function buildSchoolColumns(
  options: BuildSchoolColumnsOptions = {},
): DataTableColumn<SchoolSummary>[] {
  const { showOrgColumn = false } = options;
  const columns: DataTableColumn<SchoolSummary>[] = [
    {
      key: "name",
      header: "Schule",
      render: (row) => (
        <div>
          <div className="flex items-center gap-2">
            <span className="font-semibold text-gray-900">{row.name}</span>
            {row.hidden && (
              <span className="bg-moto-amber/15 text-moto-amber-strong rounded-full px-2 py-0.5 text-xs font-medium">
                Verborgen
              </span>
            )}
          </div>
          <div className="font-mono text-xs text-gray-500">{row.subdomain}</div>
        </div>
      ),
      sortValue: (row) => row.name.toLowerCase(),
    },
  ];

  if (showOrgColumn) {
    columns.push({
      key: "traeger",
      header: "Träger",
      render: (row) => (
        <span className="text-gray-700">{row.organizationName}</span>
      ),
      sortValue: (row) => row.organizationName.toLowerCase(),
    });
  }

  columns.push(
    {
      key: "konten",
      header: "Konten",
      align: "right",
      render: (row) => formatCount(row.kontenCount),
      sortValue: (row) => row.kontenCount,
    },
    {
      key: "geraete",
      header: "Geräte",
      align: "right",
      render: (row) => formatCount(row.geraeteCount),
      sortValue: (row) => row.geraeteCount,
    },
    {
      key: "personen",
      header: "Personen",
      align: "right",
      render: (row) => formatCount(row.personenCount),
      sortValue: (row) => row.personenCount,
    },
    {
      key: "status",
      header: "Status",
      render: (row) => <DataTableStatusBadge active={row.active} />,
      sortValue: (row) => (row.active ? 0 : 1),
    },
  );

  return columns;
}
