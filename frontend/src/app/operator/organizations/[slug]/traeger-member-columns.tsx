import type { DataTableColumn } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  TRAEGER_MEMBER_STATUS_LABELS,
  TRAEGER_ROLE_LABELS,
  type TraegerMember,
} from "~/lib/operator/provisioning-helpers";

/**
 * Spalten der Personen des Träger-Büros mit Zugang zum Träger-Portal (#4649).
 * Die Zeilenaktion setzt den zweiten Faktor zurück (#4651).
 */
export function traegerMemberColumns(
  onResetMfa: (member: TraegerMember) => void,
): DataTableColumn<TraegerMember>[] {
  return [
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
    {
      key: "actions",
      header: "",
      render: (member) => (
        <OverflowMenu
          ariaLabel={`Aktionen für ${member.email}`}
          triggerSize="sm"
          items={[
            {
              label: "Zwei-Faktor zurücksetzen",
              onClick: () => onResetMfa(member),
            },
          ]}
        />
      ),
    },
  ];
}
