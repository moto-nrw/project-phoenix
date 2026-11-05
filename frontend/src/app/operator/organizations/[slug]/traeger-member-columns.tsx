import type { DataTableColumn } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { formatDate, formatTime } from "~/lib/date-helpers";
import {
  TRAEGER_MEMBER_STATUS_LABELS,
  TRAEGER_ROLE_LABELS,
  type TraegerInvitation,
  type TraegerMember,
  type TraegerRole,
} from "~/lib/operator/provisioning-helpers";

/**
 * Eine Zeile der Träger-Büro-Liste: eine Person mit Zugang zum Träger-Portal
 * (#4649) oder eine offene Einladung, die noch niemand angenommen hat (#5749).
 */
export type TraegerBueroRow =
  | { readonly kind: "member"; readonly member: TraegerMember }
  | { readonly kind: "invitation"; readonly invitation: TraegerInvitation };

export function traegerBueroRows(
  members: readonly TraegerMember[],
  invitations: readonly TraegerInvitation[],
): TraegerBueroRow[] {
  return [
    ...members.map((member) => ({ kind: "member" as const, member })),
    ...invitations.map((invitation) => ({
      kind: "invitation" as const,
      invitation,
    })),
  ];
}

export function traegerBueroRowKey(row: TraegerBueroRow): string {
  return row.kind === "member"
    ? `member-${row.member.accountId}`
    : `invitation-${row.invitation.id}`;
}

function rowEmail(row: TraegerBueroRow): string {
  return row.kind === "member" ? row.member.email : row.invitation.email;
}

function rowRoles(row: TraegerBueroRow): readonly TraegerRole[] {
  return row.kind === "member" ? row.member.roles : row.invitation.roles;
}

function rowStatus(row: TraegerBueroRow): {
  label: string;
  tone: StatusBadgeTone;
} {
  if (row.kind === "invitation") {
    return row.invitation.expired
      ? { label: "Abgelaufen", tone: "orange" }
      : { label: "Eingeladen", tone: "blue" };
  }
  return {
    label: TRAEGER_MEMBER_STATUS_LABELS[row.member.status],
    tone: row.member.status === "active" ? "green" : "gray",
  };
}

export interface TraegerBueroActions {
  /** Setzt den zweiten Faktor einer Person zurück (#4651). */
  readonly onResetMfa: (member: TraegerMember) => void;
  /** Schickt die Einladung mit neuem Link noch einmal (#5749). */
  readonly onResend: (invitation: TraegerInvitation) => void;
  /** Nimmt die Einladung zurück, nach einer Rückfrage (#5749). */
  readonly onRevoke: (invitation: TraegerInvitation) => void;
}

/**
 * Spalten der Träger-Büro-Liste: Personen mit Zugang und offene Einladungen in
 * einer Tabelle. Eine Einladung zeigt, bis wann ihr Link gilt; ist er
 * abgelaufen, schickt „Einladung erneut senden“ einen neuen.
 */
export function traegerBueroColumns(
  actions: TraegerBueroActions,
): DataTableColumn<TraegerBueroRow>[] {
  return [
    {
      key: "email",
      header: "E-Mail",
      render: (row) => (
        <span className="font-medium wrap-anywhere text-gray-900">
          {rowEmail(row)}
        </span>
      ),
      sortValue: rowEmail,
      stacked: "title",
    },
    {
      key: "roles",
      header: "Rollen",
      render: (row) => {
        const roles = rowRoles(row);
        return roles.length > 0
          ? roles.map((role) => TRAEGER_ROLE_LABELS[role]).join(", ")
          : "–";
      },
      stacked: "field",
    },
    {
      key: "status",
      header: "Status",
      render: (row) => {
        const status = rowStatus(row);
        return <StatusBadge tone={status.tone} label={status.label} />;
      },
      sortValue: (row) => rowStatus(row).label,
      stacked: "field",
    },
    {
      key: "expires",
      header: "Link gilt bis",
      render: (row) =>
        row.kind === "invitation"
          ? `${formatDate(row.invitation.expiresAt)}, ${formatTime(row.invitation.expiresAt)} Uhr`
          : "–",
      sortValue: (row) =>
        row.kind === "invitation" ? row.invitation.expiresAt : null,
      stacked: "field",
    },
    {
      key: "actions",
      header: "",
      render: (row) =>
        row.kind === "member" ? (
          <OverflowMenu
            ariaLabel={`Aktionen für ${row.member.email}`}
            triggerSize="sm"
            items={[
              {
                label: "Zwei-Faktor zurücksetzen",
                onClick: () => actions.onResetMfa(row.member),
              },
            ]}
          />
        ) : (
          <OverflowMenu
            ariaLabel={`Aktionen für die Einladung an ${row.invitation.email}`}
            triggerSize="sm"
            items={[
              {
                label: "Einladung erneut senden",
                onClick: () => actions.onResend(row.invitation),
              },
              {
                label: "Einladung zurücknehmen",
                destructive: true,
                onClick: () => actions.onRevoke(row.invitation),
              },
            ]}
          />
        ),
      align: "right",
      stacked: "meta",
    },
  ];
}
