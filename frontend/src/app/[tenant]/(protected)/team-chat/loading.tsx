"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { Skeleton } from "~/components/ui/skeleton";
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
        description={<Skeleton className="h-4 w-52" />}
      >
        <PageHeaderWithSearch
          embedded
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
