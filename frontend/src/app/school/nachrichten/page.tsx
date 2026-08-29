"use client";

// Nachrichten im Schul-Portal ("moto schule", #2208): der Team-Chat der OGS,
// aus Sicht einer Lehrkraft. Dieselbe Oberfläche wie /team-chat im OGS-Portal,
// gebunden an die school-Session.

import { TeamChatInbox } from "~/components/messaging/team-chat-inbox";
import { useSchoolTeamChatPortal } from "~/lib/hooks/use-school-team-chat-portal";

export default function SchoolMessagesPage() {
  const portal = useSchoolTeamChatPortal();
  return <TeamChatInbox portal={portal} />;
}
