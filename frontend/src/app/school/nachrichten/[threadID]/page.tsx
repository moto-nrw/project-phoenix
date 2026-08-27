"use client";

// Eine Unterhaltung im Schul-Portal (#2208).

import { useParams, useRouter } from "next/navigation";
import { useSession } from "next-auth/react";
import { ArrowLeft } from "lucide-react";
import { TeamChatThread } from "~/components/messaging/team-chat-thread";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import {
  SCHOOL_MESSAGES_ROUTE,
  useSchoolTeamChatPortal,
} from "~/lib/hooks/use-school-team-chat-portal";
import { schoolPath } from "~/lib/school-url";

function SchoolMessagesBackNav() {
  const router = useRouter();
  const inbox = schoolPath(SCHOOL_MESSAGES_ROUTE);
  return (
    <>
      <MobileBackButton href={inbox} ariaLabel="Zurück zu Nachrichten" />
      <button
        type="button"
        onClick={() => router.push(inbox)}
        className="mb-4 hidden items-center gap-1 text-sm text-gray-500 hover:text-gray-900 md:flex"
      >
        <ArrowLeft className="h-4 w-4" /> Zurück zu Nachrichten
      </button>
    </>
  );
}

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
      backNav={<SchoolMessagesBackNav />}
    />
  );
}
