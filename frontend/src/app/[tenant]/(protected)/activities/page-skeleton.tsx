"use client";

import { Plus } from "lucide-react";
import { Button } from "~/components/ui/button";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { PageIntro } from "~/components/ui/page-intro";
import { Skeleton } from "~/components/ui/skeleton";

// Content-shaped placeholder mirroring the activity row cards, so there is no
// layout shift once the list loads.
export function ActivitiesSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      aria-label="Aktivitäten werden geladen"
      data-testid="activities-skeleton"
      className="w-full"
    >
      {/* Der Kopf rendert sofort, nur die Liste skeletonisiert. */}
      <PageIntro
        title="Aktivitäten"
        description={<Skeleton className="h-4 w-48" />}
        className="mb-6"
        actions={
          <Button
            type="button"
            variant="primary"
            size="md"
            className="gap-2"
            disabled
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Aktivität erstellen
          </Button>
        }
      >
        <PageHeaderWithSearch
          embedded
          title=""
          search={{
            value: "",
            onChange: () => {
              /* Suche ist im Ladezustand inaktiv. */
            },
            inputProps: { disabled: true },
          }}
        />
      </PageIntro>
      <div className="space-y-3">
        {[0, 1, 2, 3].map((item) => (
          <div
            key={item}
            className="moto-content-surface rounded-2xl border border-gray-200 p-5"
          >
            <div className="flex items-center justify-between">
              <div className="min-w-0 flex-1 space-y-2">
                <Skeleton className="h-5 w-2/5 rounded-full" />
                <Skeleton className="h-3.5 w-1/3 rounded-full" />
              </div>
              <Skeleton className="ml-4 h-10 w-10 flex-shrink-0 rounded-full" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
