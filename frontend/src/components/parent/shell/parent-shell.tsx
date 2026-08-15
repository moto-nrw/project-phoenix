"use client";

import { useParentMealPlanEnabled } from "~/lib/hooks/use-parent-meal-plan-enabled";
import { useParentMessagesUnread } from "~/lib/hooks/use-parent-messages-unread";
import { useParentNewsEnabled } from "~/lib/hooks/use-parent-news-enabled";
import { useParentNewsUnread } from "~/lib/hooks/use-parent-news-unread";
import { ParentBottomNav } from "./parent-bottom-nav";
import { ParentHeader } from "./parent-header";
import { ParentSidebar } from "./parent-sidebar";

/**
 * Die Huelle der Eltern-App.
 *
 * Loest die geteilte AppShell des Personal-Portals ab. Zaehler und
 * Sichtbarkeiten werden hier einmal geladen und an beide Navigationen
 * gereicht; die Navigationen selbst holen keine Daten.
 */
export function ParentShell({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  const { unreadCount: messagesUnread } = useParentMessagesUnread(true);
  const { unreadCount: newsUnread } = useParentNewsUnread(true);
  const newsEnabled = useParentNewsEnabled(true);
  const mealPlanEnabled = useParentMealPlanEnabled(true);

  const badges = { messages: messagesUnread, news: newsUnread };
  const gates = { news: newsEnabled, mealPlan: mealPlanEnabled };

  return (
    <div className="relative min-h-screen">
      <div
        className="moto-dotted-background moto-dotted-background--app-fixed moto-dotted-background--fullscreen pointer-events-none z-0"
        aria-hidden="true"
      />

      <ParentHeader />

      <div className="relative z-10 flex">
        <ParentSidebar badges={badges} gates={gates} />

        {/* Das untere Polster haelt den Inhalt ueber der Bottom-Navigation
            (56px Leiste plus Sicherheitsbereich). Ab lg traegt die
            Seitennavigation, dann faellt das Polster weg. */}
        <main className="min-w-0 flex-1 p-4 pb-[calc(5rem+env(safe-area-inset-bottom))] sm:p-6 sm:pb-[calc(5rem+env(safe-area-inset-bottom))] lg:p-8 lg:pb-8">
          {/* Ab 1280px begrenzt, damit auf breiten Schirmen keine leeren
              Bildschirmhaelften entstehen. */}
          <div className="xl:max-w-6xl">{children}</div>
        </main>
      </div>

      <ParentBottomNav badges={badges} gates={gates} />
    </div>
  );
}
