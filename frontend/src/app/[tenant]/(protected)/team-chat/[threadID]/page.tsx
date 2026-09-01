"use client";

import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { ArrowLeft } from "lucide-react";
import { BackButton } from "~/components/ui/back-button";
import { TeamChatThread } from "~/components/messaging/team-chat-thread";
import { useTenantRouter } from "~/lib/tenant-router";
import { useTenantTeamChatPortal } from "~/lib/hooks/use-tenant-team-chat-portal";

// Back navigation, in ONE place (it renders in both the not-found and the
// loaded branches). Mobile uses the kit BackButton (md:hidden); desktop gets an
// inline link because the kit has no desktop back component and this screen has
// no breadcrumb. The two are responsive-exclusive.
function TeamChatBackNav() {
  const router = useTenantRouter();
  return (
    <>
      <BackButton referrer="/team-chat" />
      <button
        type="button"
        onClick={() => router.push("/team-chat")}
        className="mb-4 hidden items-center gap-1 text-sm text-gray-500 hover:text-gray-900 md:flex"
      >
        <ArrowLeft className="h-4 w-4" /> Zurück zum Team-Chat
      </button>
    </>
  );
}

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
      backNav={<TeamChatBackNav />}
    />
  );
}
