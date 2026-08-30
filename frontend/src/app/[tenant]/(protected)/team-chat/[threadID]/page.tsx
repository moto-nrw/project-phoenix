"use client";

import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { BackButton } from "~/components/ui/back-button";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  TeamChatThread,
  type TeamChatThreadParts,
} from "~/components/messaging/team-chat-thread";
import { useTenantTeamChatPortal } from "~/lib/hooks/use-tenant-team-chat-portal";

/**
 * Die Hülle des OGS-Portals um das geteilte Chat-Fenster. Der Titel der
 * Kopfkarte ist der Name des Gegenübers, die Statuszeile zählt die
 * Nachrichten; Aus-Zustand und Fehler kommen aus dem Gerüst.
 */
function renderThreadFrame(parts: TeamChatThreadParts) {
  if (parts.state === "disabled") {
    return <TenantPage title="Team-Chat" empty={parts.empty} />;
  }
  if (parts.state === "error") {
    return (
      <TenantPage title="Unterhaltung" error={parts.errorMessage}>
        {parts.backNav}
      </TenantPage>
    );
  }
  return (
    <TenantPage
      title={
        parts.roleLabel ? `${parts.title} · ${parts.roleLabel}` : parts.title
      }
      stats={parts.stats}
      statsLoading={parts.statsLoading}
    >
      {parts.backNav}

      <div
        ref={parts.containerRef}
        className="flex min-h-[20rem] w-full flex-col overflow-hidden"
      >
        <SectionCard
          className="flex min-h-0 flex-1 flex-col"
          bodyClassName="flex min-h-0 flex-1 flex-col"
        >
          {parts.body}
        </SectionCard>
      </div>
    </TenantPage>
  );
}

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
      backNav={<BackButton referrer="/team-chat" />}
      frame={renderThreadFrame}
    />
  );
}
