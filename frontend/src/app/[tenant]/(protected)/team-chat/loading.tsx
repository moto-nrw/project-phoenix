"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { TeamChatSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: die echte Kopfkarte rendert sofort (Kicker, Titel
 * und Erklärtext sind statisch), die Suchzeile bekommt ein deaktiviertes
 * No-op-Feld, weil es hier noch keinen Seitenzustand gibt; nur die Liste
 * skeletonisiert.
 */
export default function TeamChatLoading() {
  return (
    <div className="w-full space-y-6">
      <PageIntro
        title="Team-Chat"
        description="Unterhaltungen im Kollegium, ein Verlauf je Person."
      >
        <PageHeaderWithSearch
          title=""
          search={{
            value: "",
            onChange: () => {},
            inputProps: { disabled: true },
          }}
        />
      </PageIntro>
      <TeamChatSkeleton />
    </div>
  );
}
