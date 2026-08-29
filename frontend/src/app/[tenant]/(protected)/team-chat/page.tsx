"use client";

import { TeamChatInbox } from "~/components/messaging/team-chat-inbox";
import { useTenantTeamChatPortal } from "~/lib/hooks/use-tenant-team-chat-portal";

export default function TeamChatPage() {
  const portal = useTenantTeamChatPortal();
  return <TeamChatInbox portal={portal} />;
}
