"use client";

import { Suspense } from "react";
import { useSession } from "next-auth/react";

import { EntryPointCard } from "~/components/help/guide-components";
import { TenantPage } from "~/components/ui/tenant-page";
import { hasPermission, hasRole } from "~/lib/auth-utils";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";
import { formatStatusDate } from "~/lib/date-helpers";
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
  // before the header, so the real page header renders immediately and only the
  // card grid skeletonizes.
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
    // Die Elternanfragen-Kachel ist entfallen: die Freigabeansicht lebt seit
    // #2429 im Top-Level-Modul "Anfragen" (eigener Sidebar-Eintrag).
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
    // Die Übersicht lädt selbst keine Zahlen (Nachrichten und Konto-Anfragen
    // liegen hinter eigenen Seiten), deshalb steht in der Statuszeile der
    // Berliner Kalendertag.
    <TenantPage
      title="Eltern"
      stats={formatStatusDate()}
      statsLoading={showSkeleton}
      // Laden und Leerstand kommen aus dem Gerüst, nicht aus einem eigenen
      // Skelett im Inhalt.
      loading={showSkeleton}
      empty={
        !showSkeleton && visibleCards.length === 0
          ? {
              title: "Für Sie ist hier noch nichts freigeschaltet",
              description:
                "Der Elternbereich wird von der Schulleitung freigeschaltet. Bitten Sie eine Person mit Administrationsrechten darum.",
            }
          : null
      }
    >
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
    </TenantPage>
  );
}

export default function ElternPage() {
  return (
    <Suspense
      fallback={
        // Auch im Suspense-Fallback steht die Kopfkarte schon da: der Titel
        // ist statisch, nur das Kachelraster skelettiert.
        <TenantPage title="Eltern" stats={formatStatusDate()} loading />
      }
    >
      <ElternContent />
    </Suspense>
  );
}
