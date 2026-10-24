import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  TRAEGER_MEMBER_STATUS_LABELS,
  TRAEGER_ROLE_LABELS,
  type TraegerMember,
} from "~/lib/operator/provisioning-helpers";

const COLUMNS: DataTableColumn<TraegerMember>[] = [
  {
    key: "email",
    header: "E-Mail",
    render: (member) => (
      <span className="font-medium text-gray-900">{member.email}</span>
    ),
    sortValue: (member) => member.email,
  },
  {
    key: "roles",
    header: "Rollen",
    render: (member) =>
      member.roles.length > 0
        ? member.roles.map((role) => TRAEGER_ROLE_LABELS[role]).join(", ")
        : "–",
  },
  {
    key: "status",
    header: "Status",
    render: (member) => (
      <StatusBadge
        tone={member.status === "active" ? "green" : "gray"}
        label={TRAEGER_MEMBER_STATUS_LABELS[member.status]}
      />
    ),
    sortValue: (member) => member.status,
  },
];

/** Personen des Träger-Büros mit Zugang zum Träger-Portal (#4649). */
export function TraegerMembersTable({
  members,
  isLoading,
}: {
  readonly members: TraegerMember[];
  readonly isLoading: boolean;
}) {
  return (
    <DataTable
      columns={COLUMNS}
      rows={members}
      getRowKey={(member) => member.accountId}
      defaultSortKey="email"
      isLoading={isLoading}
    />
  );
}
