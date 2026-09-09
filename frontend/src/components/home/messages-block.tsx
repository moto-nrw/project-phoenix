"use client";

import { EmptyState } from "~/components/ui/empty-state";
import Link from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { HOME_CARD_BODY, HomeCardIcon } from "~/components/home/home-card";
import { HomeCardLink } from "~/components/home/home-card-rows";
import { useMessagesUnread } from "~/lib/hooks/use-messages-unread";
import { useStaffMessagesUnread } from "~/lib/hooks/use-staff-messages-unread";
import { useTenantSafe } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Baustein „Ungelesene Nachrichten" (#2180): was in den Posteingängen noch
 * auf jemanden wartet — Nachrichten von Eltern und aus dem Team-Chat.
 *
 * Die Zähler kommen aus denselben Hooks wie die Zähler in der Seitenleiste;
 * eine zweite Quelle würde zwei verschiedene Zahlen für dieselbe Sache
 * zeigen. Jede Zeile führt in den Posteingang, in dem man antwortet.
 */
export function MessagesBlock() {
  const tenantPath = useTenantAwarePath();
  const tenant = useTenantSafe()?.tenant;
  const parentsEnabled = tenant?.messagingEnabled === true;
  const teamEnabled = tenant?.staffMessagingEnabled === true;
  const parents = useMessagesUnread();
  const team = useStaffMessagesUnread();

  const rows = [
    {
      key: "parents",
      enabled: parentsEnabled,
      label: "Nachrichten von Eltern",
      hint: "Ungelesene Nachrichten im Posteingang",
      count: parents.unreadCount,
      href: tenantPath("/messages"),
    },
    {
      key: "team",
      enabled: teamEnabled,
      label: "Team-Chat",
      hint: "Ungelesene Nachrichten im Team",
      count: team.unreadCount,
      href: tenantPath("/team-chat"),
    },
  ].filter((row) => row.enabled && row.count > 0);

  const allHref = tenantPath(parentsEnabled ? "/messages" : "/team-chat");

  return (
    <SectionCard
      title="Ungelesene Nachrichten"
      leading={<HomeCardIcon concept="messages" />}
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      inlineActions
      actions={
        <HomeCardLink
          href={allHref}
          label="Ungelesene Nachrichten: Posteingang öffnen"
        >
          Posteingang
        </HomeCardLink>
      }
    >
      {rows.length === 0 ? (
        <EmptyState
          className="py-4"
          title="Alles gelesen"
          description="Neue Nachrichten von Eltern und aus dem Team erscheinen hier."
        />
      ) : (
        <ul className="space-y-2">
          {rows.map((row) => (
            <li key={row.key}>
              <Link
                href={row.href}
                className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
              >
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium text-gray-900">
                    {row.label}
                  </span>
                  <span className="block truncate text-xs text-gray-500">
                    {row.hint}
                  </span>
                </span>
                <StatusBadge tone="blue" label={`${row.count} ungelesen`} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </SectionCard>
  );
}
