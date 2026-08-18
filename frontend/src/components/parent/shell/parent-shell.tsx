"use client";

import useSWR from "swr";
import { useParentMealPlanEnabled } from "~/lib/hooks/use-parent-meal-plan-enabled";
import { useParentMessagesUnread } from "~/lib/hooks/use-parent-messages-unread";
import { useParentNewsEnabled } from "~/lib/hooks/use-parent-news-enabled";
import { useParentNewsUnread } from "~/lib/hooks/use-parent-news-unread";
import { Header } from "~/components/dashboard/header";
import { listMyChildren } from "~/lib/parent-api";
import { ParentBottomNav } from "./parent-bottom-nav";
import { ParentSidebar } from "./parent-sidebar";
import { useShellAuth } from "~/lib/shell-auth-context";

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
  const { status } = useShellAuth();
  const authenticated = status === "authenticated";
  const { unreadCount: messagesUnread } =
    useParentMessagesUnread(authenticated);
  const { unreadCount: newsOpen } = useParentNewsUnread(authenticated);
  const { data: linkedChildren } = useSWR(
    authenticated ? "parent-shell-children" : null,
    listMyChildren,
  );
  const newsEnabled = useParentNewsEnabled(
    authenticated,
    linkedChildren ?? null,
  );
  const mealPlanEnabled = useParentMealPlanEnabled(
    authenticated,
    linkedChildren ?? null,
  );

  const badges = { messages: messagesUnread, news: newsOpen };
  const gates = { news: newsEnabled, mealPlan: mealPlanEnabled };
  const childCount = linkedChildren?.length ?? 0;

  return (
    <div className="relative min-h-screen">
      <div
        className="moto-dotted-background moto-dotted-background--app-fixed moto-dotted-background--fullscreen pointer-events-none z-0"
        aria-hidden="true"
      />
      <div
        data-parent-safe-area-top
        className="h-[env(safe-area-inset-top)] bg-white lg:hidden"
        aria-hidden="true"
      />

      <div className="sticky top-0 z-50 hidden lg:block">
        <Header />
      </div>

      <div className="relative z-10 flex">
        <ParentSidebar badges={badges} gates={gates} childCount={childCount} />

        <main className="min-w-0 flex-1 p-4 pb-[calc(7rem+env(safe-area-inset-bottom))] md:p-8 md:pb-[calc(7rem+env(safe-area-inset-bottom))] lg:pb-8">
          <div className="relative z-10">{children}</div>
        </main>
      </div>

      <ParentBottomNav badges={badges} gates={gates} childCount={childCount} />
    </div>
  );
}
