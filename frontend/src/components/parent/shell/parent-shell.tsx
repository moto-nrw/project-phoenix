"use client";

import useSWR from "swr";
import { useParentMealPlanEnabled } from "~/lib/hooks/use-parent-meal-plan-enabled";
import { useParentMessagesUnread } from "~/lib/hooks/use-parent-messages-unread";
import { useParentNewsEnabled } from "~/lib/hooks/use-parent-news-enabled";
import { useParentNewsUnread } from "~/lib/hooks/use-parent-news-unread";
import { Header } from "~/components/dashboard/header";
import { PortalShell } from "~/components/ui/portal-shell";
import { listMyChildren } from "~/lib/parent-api";
import { ParentBottomNav } from "./parent-bottom-nav";
import { ParentSidebar } from "./parent-sidebar";
import { useShellAuth } from "~/lib/shell-auth-context";

/**
 * Die Huelle der Eltern-App.
 *
 * Loest die geteilte AppShell des Personal-Portals ab; den Rahmen liefert
 * die geteilte `PortalShell`. Zaehler und
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
    <PortalShell
      header={<Header />}
      headerClassName="sticky top-0 z-50 hidden lg:block"
      backgroundClassName="moto-dotted-background--parent"
      topLayer={
        <div
          data-parent-safe-area-top
          className="relative z-10 h-[env(safe-area-inset-top)] min-h-8 bg-transparent lg:hidden"
          aria-hidden="true"
        />
      }
      sidebar={
        <ParentSidebar badges={badges} gates={gates} childCount={childCount} />
      }
      bottomNav={
        <ParentBottomNav
          badges={badges}
          gates={gates}
          childCount={childCount}
        />
      }
    >
      {children}
    </PortalShell>
  );
}
