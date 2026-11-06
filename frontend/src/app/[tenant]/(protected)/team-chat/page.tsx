"use client";

import { TeamChatInbox } from "~/components/messaging/team-chat-inbox";
import { useTenantTeamChatPortal } from "~/lib/hooks/use-tenant-team-chat-portal";

/**
 * Posteingang des Team-Chats im OGS-Portal. Logik und Liste kommen aus der
 * geteilten `TeamChatInbox` (#2208).
 */
export default function TeamChatPage() {
  const portal = useTenantTeamChatPortal();
  return <TeamChatInbox portal={portal} />;
}
