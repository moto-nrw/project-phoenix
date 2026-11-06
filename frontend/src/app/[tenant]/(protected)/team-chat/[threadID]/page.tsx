"use client";

import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { TeamChatThread } from "~/components/messaging/team-chat-thread";
import { useTenantTeamChatPortal } from "~/lib/hooks/use-tenant-team-chat-portal";

/** Rückweg jeder Unterhaltung: der Posteingang des Team-Chats. */
const TEAM_CHAT_BACK = {
  href: "/team-chat",
  label: "Zurück zum Team-Chat",
} as const;

/** Eine Unterhaltung im OGS-Portal; Logik in der geteilten `TeamChatThread`. */
export default function TeamThreadPage() {
  const params = useParams();
  const threadID = params.threadID as string;
  const { data: session } = useSession();
  const portal = useTenantTeamChatPortal();
  // Which bubbles are "mine". The backend stamps every message with its sender
  // account, and the session carries the viewer's — so the side a bubble sits
  // on is decided here, not by the API.
  const myAccountId = session?.user?.id ?? "";

  return (
    <TeamChatThread
      portal={portal}
      threadID={threadID}
      myAccountId={myAccountId}
      back={TEAM_CHAT_BACK}
    />
  );
}
