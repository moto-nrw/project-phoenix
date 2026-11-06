"use client";

import { MessagesSquare } from "lucide-react";
import { formErrorDetail, formErrorMessage } from "~/components/ui/form-error";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import type {
  TeamChatInboxParts,
  TeamChatThreadParts,
} from "~/components/messaging/team-chat-frame-parts";

/**
 * Die Hülle um den geteilten Posteingang, im OGS- wie im Schul-Portal (#5604): Kopfkarte mit
 * Statuszeile, Suche, Filter und Hauptaktion kommen aus dem Gerüst.
 */
export function renderInboxFrame(parts: TeamChatInboxParts) {
  return (
    <TenantPage
      title={parts.title}
      stats={parts.stats}
      statsLoading={parts.statsLoading}
      badge={
        parts.loading || parts.count === null
          ? undefined
          : { icon: <MessagesSquare size={20} />, count: parts.count }
      }
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
      error={parts.error}
      empty={parts.empty}
      overlays={parts.overlays}
    >
      {parts.staleWarning}
      {parts.list}
    </TenantPage>
  );
}

/**
 * Die Hülle um das geteilte Chat-Fenster, im OGS- wie im Schul-Portal (#5604). Der Titel der
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
      info={parts.audience}
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
