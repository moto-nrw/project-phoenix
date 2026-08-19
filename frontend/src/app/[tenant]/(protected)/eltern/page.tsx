"use client";

import { Suspense } from "react";
import { useSession } from "next-auth/react";

import { EntryPointCard } from "~/components/help/guide-components";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import {
  SkeletonRegion,
  CardGridSkeleton,
} from "~/components/ui/page-skeletons";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { hasPermission, hasRole } from "~/lib/auth-utils";
import { canReviewChangeRequests } from "~/lib/change-request-access";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";
import { useTenantAwarePath } from "~/lib/tenant-path";
import type { MotoConceptKey } from "~/lib/moto-concepts";

interface ElternCard {
  readonly href: string;
  readonly title: string;
  readonly body: string;
  readonly concept: MotoConceptKey;
  readonly points: readonly string[];
  readonly show: boolean;
}

function ElternContent() {
  const { data: session, status } = useSession({ required: true });
  const isMobile = useIsMobile();
  // Card links must carry the tenant segment in path routing mode (e.g.
  // /school-a/messages); bare hrefs would leave the tenant path entirely.
  const tenantPath = useTenantAwarePath();

  const userIsAdmin = hasRole(session, "admin");
  // Mirror the sidebar gating so a card never links somewhere the caller would
  // be 403'd on. The feature flags live in the settings schema; only fetch it
  // for callers the backend lets read config (admins + config:read holders).
  const canReadConfig = userIsAdmin || hasPermission(session, "config:read");
  const { data: settingsSchema } = useSettingsSchema(canReadConfig, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
  });

  const parentNewsEnabled =
    getSettingValue(settingsSchema, "operations.parent_news_enabled") === true;
  const mealPlanEnabled =
    getSettingValue(settingsSchema, "operations.meal_plan_enabled") === true;
  // Elternmitteilungen authoring is admin-only in v1 (admin:* wildcard guards
  // every /api/parent-announcements route). Same rule as the sidebar entry.
  const canAnnounce = hasPermission(session, "admin:*");

  // Auth-loading joins the showSkeleton flag instead of an early return
  // before the header, so the real PageHeaderWithSearch and intro copy
  // render immediately and only the card grid skeletonizes.
  const showSkeleton = status === "loading";

  const cards: readonly ElternCard[] = [
    {
      href: "/messages",
      title: "Nachrichten",
      body: "Unterhaltungen mit Eltern lesen und beantworten.",
      concept: "parentConversations",
      points: ["Ein Verlauf pro Kind", "Rückfragen direkt klären"],
      show: true,
    },
    {
      href: "/admin/guardian-approvals",
      title: "Konto-Anfragen",
      body: "Zugänge von Eltern zum Elternportal freigeben.",
      concept: "accounts",
      points: ["Neue Anfragen prüfen", "Konten bestätigen"],
      show: userIsAdmin,
    },
    {
      href: "/admin/change-requests",
      title: "Änderungsanfragen",
      body: "Wünsche der Eltern zu Betreuungszeiten und Stammdaten bearbeiten.",
      concept: "changeHistory",
      points: ["Betreuungszeiten anpassen", "Stammdaten aktualisieren"],
      show: canReviewChangeRequests(session),
    },
    {
      href: "/parent-announcements",
      title: "Elternmitteilungen",
      body: "Neuigkeiten an alle Eltern senden.",
      concept: "parentMessages",
      points: ["Mitteilungen verfassen", "Benachrichtigung per E-Mail"],
      show: canAnnounce && parentNewsEnabled,
    },
    {
      href: "/meal-plan",
      title: "Essensplan",
      body: "Den Speiseplan pflegen, den Eltern im Portal sehen.",
      concept: "mealPlan",
      points: ["Gerichte je Woche", "Für Eltern sichtbar"],
      show:
        mealPlanEnabled &&
        (userIsAdmin || hasPermission(session, "config:read")),
    },
  ];

  const visibleCards = cards.filter((card) => card.show);

  return (
    <div className="w-full">
      {isMobile && <PageHeaderWithSearch title="Eltern" />}

      <div className="min-h-[60vh]">
        {/* Intro — carries the Anleitung/Help-Guide design language (green
            eyebrow + short lead) into the in-app overview. */}
        <div className="mb-8 hidden max-w-2xl lg:block">
          <p className="text-moto-green-strong text-sm font-bold tracking-[0.08em] uppercase">
            Elternbereich
          </p>
          <p className="mt-3 text-base leading-7 text-gray-600">
            Alles rund um die Kommunikation mit den Eltern an einem Ort.
            Nachrichten, Anfragen, Mitteilungen und der Essensplan.
          </p>
        </div>

        {showSkeleton ? (
          <ElternCardsSkeleton />
        ) : (
          <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {visibleCards.map((card) => (
              <EntryPointCard
                key={card.href}
                href={tenantPath(card.href)}
                title={card.title}
                body={card.body}
                concept={card.concept}
                iconTone="blue"
                points={card.points}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * Data-region skeleton: just the entry-point card grid. The real header
 * (PageHeaderWithSearch on mobile, intro copy on desktop) renders
 * immediately regardless of loading state — only this data-bound region
 * skeletonizes while the session/settings load.
 */
function ElternCardsSkeleton() {
  return (
    <SkeletonRegion label="Elternbereich wird geladen…">
      <CardGridSkeleton cards={4} rowsPerCard={2} />
    </SkeletonRegion>
  );
}

export default function ElternPage() {
  return (
    <Suspense fallback={<ElternCardsSkeleton />}>
      <ElternContent />
    </Suspense>
  );
}
