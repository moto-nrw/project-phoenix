"use client";

import { useState, useCallback, useMemo } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { createLogger } from "~/lib/logger";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import {
  SimpleEmptyState,
  CardSkeletons,
} from "~/app/operator/provisioning/provisioning-shared";
import {
  listOperators,
  listPendingOperatorInvitations,
  resendOperatorInvitation,
  revokeOperatorInvitation,
} from "~/lib/operator/invitation-api";
import type {
  OperatorListItem,
  PendingOperatorInvitation,
} from "~/lib/operator/invitation-api";
import { getRelativeTime } from "~/lib/format-utils";
import { InviteOperatorModal } from "./invite-operator-modal";

const logger = createLogger({ component: "OperatorsManagementPage" });

type ActiveTab = "operators" | "invitations";

export function OperatorsManagementPage() {
  const [activeTab, setActiveTab] = useState<ActiveTab>("operators");
  const [inviteOpen, setInviteOpen] = useState(false);
  const [refreshKey, setRefreshKey] = useState(0);

  const { data: operators, isLoading: operatorsLoading } = useSWR(
    ["operator-list", refreshKey],
    () => listOperators(),
  );

  const { data: invitations, isLoading: invitationsLoading } = useSWR(
    ["operator-invitations", refreshKey],
    () => listPendingOperatorInvitations(),
  );

  const refresh = useCallback(() => setRefreshKey((k) => k + 1), []);

  const handleResend = useCallback(
    async (id: number) => {
      try {
        await resendOperatorInvitation(id);
        refresh();
      } catch (error) {
        logger.error("resend_invitation_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [refresh],
  );

  const handleRevoke = useCallback(
    async (id: number) => {
      try {
        await revokeOperatorInvitation(id);
        refresh();
      } catch (error) {
        logger.error("revoke_invitation_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [refresh],
  );

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "operators",
          label: "Operatoren",
          count: operators?.length,
        },
        {
          id: "invitations",
          label: "Einladungen",
          count: invitations?.length,
        },
      ],
      activeTab,
      onTabChange: (tabId: string) => {
        setActiveTab(tabId as ActiveTab);
      },
    }),
    [activeTab, operators?.length, invitations?.length],
  );

  const isLoading =
    activeTab === "operators" ? operatorsLoading : invitationsLoading;

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Operatoren"
        tabs={tabs}
        actionButton={
          <button
            type="button"
            onClick={() => setInviteOpen(true)}
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
          >
            Einladen
          </button>
        }
        mobileActionButton={
          <button
            type="button"
            onClick={() => setInviteOpen(true)}
            className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
            aria-label="Operator einladen"
          >
            <PlusIcon />
          </button>
        }
      />

      {isLoading && <CardSkeletons />}

      {!isLoading && activeTab === "operators" && (
        <>
          {operators && operators.length > 0 ? (
            <OperatorsTable operators={operators} />
          ) : (
            <SimpleEmptyState
              title="Keine Operatoren"
              description="Es sind noch keine Operator-Konten vorhanden."
            />
          )}
        </>
      )}

      {!isLoading && activeTab === "invitations" && (
        <>
          {invitations && invitations.length > 0 ? (
            <InvitationsTable
              invitations={invitations}
              onResend={handleResend}
              onRevoke={handleRevoke}
            />
          ) : (
            <SimpleEmptyState
              title="Keine Einladungen"
              description="Es gibt keine ausstehenden Einladungen."
            />
          )}
        </>
      )}

      <InviteOperatorModal
        isOpen={inviteOpen}
        onClose={() => setInviteOpen(false)}
        onSuccess={() => {
          setInviteOpen(false);
          refresh();
        }}
      />
    </div>
  );
}

// --- Sort utilities (same pattern as provisioning page) ---

type SortDirection = "asc" | "desc";

interface SortState<K extends string> {
  key: K;
  direction: SortDirection;
}

function useSort<K extends string>(
  defaultKey: K,
  defaultDirection: SortDirection = "asc",
) {
  const [sort, setSort] = useState<SortState<K>>({
    key: defaultKey,
    direction: defaultDirection,
  });

  const toggle = useCallback((key: K) => {
    setSort((prev) =>
      prev.key === key
        ? { key, direction: prev.direction === "asc" ? "desc" : "asc" }
        : { key, direction: "asc" },
    );
  }, []);

  return { sort, toggle };
}

function SortIndicator({
  active,
  direction,
}: {
  readonly active: boolean;
  readonly direction: SortDirection;
}) {
  return (
    <span
      className={`ml-1 inline-block ${active ? "text-gray-700" : "text-gray-300"}`}
    >
      {active ? (direction === "asc" ? "↑" : "↓") : "↕"}
    </span>
  );
}

function SortableHeader<K extends string>({
  label,
  sortKey,
  sort,
  onToggle,
}: {
  readonly label: string;
  readonly sortKey: K;
  readonly sort: SortState<K>;
  readonly onToggle: (key: K) => void;
}) {
  return (
    <th
      className="cursor-pointer px-5 py-3 transition-colors select-none hover:text-gray-700"
      onClick={() => onToggle(sortKey)}
    >
      {label}
      <SortIndicator active={sort.key === sortKey} direction={sort.direction} />
    </th>
  );
}

// --- Operators Table ---

type OperatorSortKey = "name" | "email" | "lastLogin";

function getOperatorSortValue(op: OperatorListItem, key: string): string {
  switch (key) {
    case "name":
      return op.display_name.toLowerCase();
    case "email":
      return op.email.toLowerCase();
    case "lastLogin":
      return op.last_login ?? "";
    default:
      return "";
  }
}

function sortOperators(
  operators: OperatorListItem[],
  sort: SortState<OperatorSortKey>,
): OperatorListItem[] {
  return [...operators].sort((a, b) => {
    const aVal = getOperatorSortValue(a, sort.key);
    const bVal = getOperatorSortValue(b, sort.key);
    const cmp = aVal.localeCompare(bVal);
    return sort.direction === "asc" ? cmp : -cmp;
  });
}

function OperatorsTable({
  operators,
}: {
  readonly operators: OperatorListItem[];
}) {
  const { sort, toggle } = useSort<OperatorSortKey>("name");
  const sorted = useMemo(
    () => sortOperators(operators, sort),
    [operators, sort],
  );

  return (
    <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
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
              label="Letzter Login"
              sortKey="lastLogin"
              sort={sort}
              onToggle={toggle}
            />
          </tr>
        </thead>
        <tbody>
          {sorted.map((op) => (
            <tr key={op.id} className="border-b border-gray-50 last:border-0">
              <td className="px-5 py-3 font-medium text-gray-900">
                {op.display_name}
              </td>
              <td className="px-5 py-3 text-gray-600">{op.email}</td>
              <td className="px-5 py-3 text-gray-600">
                {op.last_login ? getRelativeTime(op.last_login) : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// --- Invitations Table ---

type InvitationSortKey = "email" | "inviter" | "expiresAt";

function getInvitationSortValue(
  inv: PendingOperatorInvitation,
  key: string,
): string {
  switch (key) {
    case "email":
      return inv.email.toLowerCase();
    case "inviter":
      return inv.inviter_name.toLowerCase();
    case "expiresAt":
      return inv.expires_at;
    default:
      return "";
  }
}

function sortInvitations(
  invitations: PendingOperatorInvitation[],
  sort: SortState<InvitationSortKey>,
): PendingOperatorInvitation[] {
  return [...invitations].sort((a, b) => {
    const aVal = getInvitationSortValue(a, sort.key);
    const bVal = getInvitationSortValue(b, sort.key);
    const cmp = aVal.localeCompare(bVal);
    return sort.direction === "asc" ? cmp : -cmp;
  });
}

function InvitationsTable({
  invitations,
  onResend,
  onRevoke,
}: {
  readonly invitations: PendingOperatorInvitation[];
  readonly onResend: (id: number) => void;
  readonly onRevoke: (id: number) => void;
}) {
  const { sort, toggle } = useSort<InvitationSortKey>("email");
  const sorted = useMemo(
    () => sortInvitations(invitations, sort),
    [invitations, sort],
  );

  return (
    <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
            <SortableHeader
              label="E-Mail"
              sortKey="email"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Eingeladen von"
              sortKey="inviter"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Läuft ab"
              sortKey="expiresAt"
              sort={sort}
              onToggle={toggle}
            />
            <th className="px-5 py-3">Aktionen</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((inv) => (
            <InvitationRow
              key={inv.id}
              invitation={inv}
              onResend={onResend}
              onRevoke={onRevoke}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function InvitationRow({
  invitation,
  onResend,
  onRevoke,
}: {
  readonly invitation: PendingOperatorInvitation;
  readonly onResend: (id: number) => void;
  readonly onRevoke: (id: number) => void;
}) {
  const expiresAt = new Date(invitation.expires_at);
  const isExpiringSoon = expiresAt.getTime() - Date.now() < 6 * 60 * 60 * 1000;

  return (
    <tr className="border-b border-gray-50 last:border-0">
      <td className="px-5 py-3 font-medium text-gray-900">
        {invitation.email}
        {invitation.display_name && (
          <span className="ml-1 text-gray-400">
            ({invitation.display_name})
          </span>
        )}
      </td>
      <td className="px-5 py-3 text-gray-600">{invitation.inviter_name}</td>
      <td className="px-5 py-3">
        <span className={isExpiringSoon ? "text-amber-600" : "text-gray-600"}>
          {formatTimeUntil(expiresAt)}
        </span>
      </td>
      <td className="px-5 py-3">
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => onResend(invitation.id)}
            className="rounded-lg border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
          >
            Erneut senden
          </button>
          <button
            type="button"
            onClick={() => onRevoke(invitation.id)}
            className="rounded-lg border border-red-200 px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 hover:text-red-700"
          >
            Widerrufen
          </button>
        </div>
      </td>
    </tr>
  );
}

function formatTimeUntil(date: Date): string {
  const diff = date.getTime() - Date.now();
  if (diff <= 0) return "abgelaufen";
  const minutes = Math.floor(diff / 60000);
  if (minutes < 60) return `in ${minutes} Min.`;
  const hours = Math.floor(minutes / 60);
  return `in ${hours} Std.`;
}

function PlusIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
    </svg>
  );
}
