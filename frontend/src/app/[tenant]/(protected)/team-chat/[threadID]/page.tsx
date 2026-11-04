"use client";

import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { formErrorDetail, formErrorMessage } from "~/components/ui/form-error";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  TeamChatThread,
  type TeamChatThreadParts,
} from "~/components/messaging/team-chat-thread";
import { useTenantTeamChatPortal } from "~/lib/hooks/use-tenant-team-chat-portal";

/** Rückweg jeder Unterhaltung: der Posteingang des Team-Chats. */
const TEAM_CHAT_BACK = {
  href: "/team-chat",
  label: "Zurück zum Team-Chat",
} as const;

/**
 * Die Hülle des OGS-Portals um das geteilte Chat-Fenster. Der Titel der
 * Kopfkarte ist der Name des Gegenübers, die Statuszeile zählt die
 * Nachrichten; Aus-Zustand und Fehler kommen aus dem Gerüst. Der Rückweg
 * steht in jedem Zustand über dem Titel.
 */
export function renderThreadFrame(parts: TeamChatThreadParts) {
  const back = {
    back: true,
    backHref: parts.back.href,
    backLabel: parts.back.label,
  } as const;
  if (parts.state === "disabled") {
    return <TenantPage title="Team-Chat" {...back} empty={parts.empty} />;
  }
  if (parts.state === "error") {
    return (
      <TenantPage
        title="Unterhaltung"
        {...back}
        // Wiederholen und Vorgangskennung kommen aus dem Ladefehler (#2517).
        error={{
          message: formErrorMessage(parts.error) ?? "",
          action: errorAlertActions(formErrorDetail(parts.error)),
        }}
      />
    );
  }
  return (
    <TenantPage
      title={
        parts.roleLabel ? `${parts.title} · ${parts.roleLabel}` : parts.title
      }
      {...back}
      stats={parts.stats}
      statsLoading={parts.statsLoading}
      actions={parts.actions ?? undefined}
    >
      <div
        ref={parts.containerRef}
        className="flex min-h-[20rem] w-full flex-col overflow-hidden"
      >
        {/* `moto-scroll-surface`: die Höhe setzt `useChatViewportLock`, nicht
            die Wachs-Regel des Seitenrumpfs; nur die Liste scrollt (#3328). */}
        <SectionCard
          className="moto-scroll-surface flex min-h-0 flex-1 flex-col"
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
      back={TEAM_CHAT_BACK}
      frame={renderThreadFrame}
    />
  );
}
