"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { Skeleton } from "~/components/ui/skeleton";
import { MessagesSkeleton } from "./page-skeleton";

/**
 * Route-level loading UI: renders the real chrome immediately (Polaris: real
 * chrome first, skeletonize only the data region). Kicker, Titel und
 * Erklärtext sind statisch, also rendert die echte Kopfkarte sofort; die
 * Suchzeile bekommt ein deaktiviertes No-op-Feld, weil es hier noch keinen
 * Seitenzustand gibt.
 */
export default function MessagesLoading() {
  return (
    <div className="w-full space-y-6">
      <PageIntro
        kicker="Eltern"
        title="Nachrichten"
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
      <MessagesSkeleton />
    </div>
  );
}
