"use client";

// Eine Unterhaltung im Schul-Portal (#2208).

import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { TeamChatThread } from "~/components/messaging/team-chat-thread";
import {
  SCHOOL_MESSAGES_ROUTE,
  useSchoolTeamChatPortal,
} from "~/lib/hooks/use-school-team-chat-portal";
import { schoolPath } from "~/lib/school-url";

export default function SchoolMessageThreadPage() {
  const params = useParams();
  const threadID = params.threadID as string;
  const { data: session } = useSession();
  const portal = useSchoolTeamChatPortal();
  const myAccountId = session?.user?.id ?? "";

  return (
    <TeamChatThread
      portal={portal}
      threadID={threadID}
      myAccountId={myAccountId}
      back={{
        href: schoolPath(SCHOOL_MESSAGES_ROUTE),
        label: "Zurück zu Nachrichten",
      }}
    />
  );
}
