"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import {
  TeamChatInbox,
  type TeamChatInboxParts,
} from "~/components/messaging/team-chat-inbox";
import { useTenantTeamChatPortal } from "~/lib/hooks/use-tenant-team-chat-portal";

/**
 * Die Hülle des OGS-Portals um den geteilten Posteingang: Kopfkarte mit
 * Statuszeile, Suche, Filter und Hauptaktion kommen aus dem Gerüst.
 */
function renderInboxFrame(parts: TeamChatInboxParts) {
  return (
    <TenantPage
      title={parts.title}
      stats={parts.stats}
      statsLoading={parts.statsLoading}
      actions={parts.composeButton ?? undefined}
      search={parts.search}
      filters={
        parts.chatEnabled
          ? [
              {
                id: "unread",
                type: "dropdown",
                label: "Unterhaltungen filtern",
                value: parts.onlyUnread ? "unread" : "all",
                onChange: (next) => parts.setOnlyUnread(next === "unread"),
                options: [
                  { value: "all", label: "Alle Unterhaltungen" },
                  { value: "unread", label: "Nur ungelesen" },
                ],
              },
            ]
          : undefined
      }
      loading={parts.loading}
      empty={parts.empty}
      overlays={parts.overlays}
    >
      {parts.staleWarning}
      {parts.list}
    </TenantPage>
  );
}

/**
 * Posteingang des Team-Chats im OGS-Portal. Logik und Liste kommen aus der
 * geteilten `TeamChatInbox` (#2208); die Seite steuert nur das Gerüst bei.
 */
export default function TeamChatPage() {
  const portal = useTenantTeamChatPortal();
  return <TeamChatInbox portal={portal} frame={renderInboxFrame} />;
}
