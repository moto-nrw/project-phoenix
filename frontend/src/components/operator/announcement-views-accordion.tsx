"use client";

import { useState, useMemo } from "react";
import { ChevronDown, Check } from "lucide-react";
import { operatorAnnouncementsService } from "~/lib/operator/announcements-api";
import type { AnnouncementViewDetail } from "~/lib/operator/announcements-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import { Avatar } from "~/components/ui/avatar";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useSWRAuth } from "~/lib/swr";
import { operatorAnnouncementKeys } from "~/lib/swr/portal-keys";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { Loading } from "~/components/ui/loading";

interface AnnouncementViewsAccordionProps {
  readonly announcementId: string;
}

export function AnnouncementViewsAccordion({
  announcementId,
}: AnnouncementViewsAccordionProps) {
  const [isOpen, setIsOpen] = useState(false);
  const {
    data: stats,
    error: statsError,
    mutate: reloadStats,
  } = useSWRAuth(
    isOpen ? operatorAnnouncementKeys.stats(announcementId) : null,
    () => operatorAnnouncementsService.fetchStats(announcementId),
    { keepPreviousData: false, refreshInterval: 30000 },
  );
  const {
    data: viewDetails,
    error,
    isLoading,
    mutate,
  } = useSWRAuth<AnnouncementViewDetail[]>(
    isOpen ? operatorAnnouncementKeys.views(announcementId) : null,
    () => operatorAnnouncementsService.fetchViewDetails(announcementId),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );
  const loadError = useSwrLoadError(
    error,
    "die Liste der Lesebestätigungen",
    () => void mutate(),
  );
  const statsLoadError = useSwrLoadError(
    statsError,
    "die Auswertung der Ankündigung",
    () => void reloadStats(),
  );
  const confirmedUsers = useMemo(
    () => (viewDetails ?? []).filter((detail) => detail.dismissed),
    [viewDetails],
  );
  const count = stats?.dismissed_count;

  return (
    <div
      className="mt-3 border-t border-gray-100"
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
      role="presentation"
    >
      {/* Accordion header */}
      <button
        type="button"
        onClick={() => setIsOpen((open) => !open)}
        aria-expanded={isOpen}
        className="flex w-full items-center justify-between py-3 text-sm text-gray-600 transition-colors hover:text-gray-900"
      >
        <span className="flex items-center gap-1.5 font-medium">
          <span>Lesebestätigungen</span>
          {count !== undefined && (
            <span className="text-xs font-normal text-gray-400">({count})</span>
          )}
        </span>
        <ChevronDown
          className={`h-4 w-4 transition-transform duration-200 ${isOpen ? "rotate-180" : ""}`}
        />
      </button>

      {/* Accordion body */}
      {isOpen && (
        <div className="overflow-hidden">
          <div className="pb-3">
            {/* Loading state */}
            {isLoading && <Loading fullPage={false} message="Laden..." />}

            {/* Error */}
            <LoadErrorAlert
              error={loadError ?? statsLoadError}
              className="mb-2"
            />

            {/* Confirmed users list */}
            {!isLoading && confirmedUsers.length > 0 && (
              <div className="space-y-0 divide-y divide-gray-100">
                {confirmedUsers.map((detail) => (
                  <div
                    key={detail.userId}
                    className="flex items-center gap-2.5 py-2.5 first:pt-0 last:pb-0"
                  >
                    <Avatar name={detail.userName} decorative />

                    {/* Content */}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between">
                        <div className="flex items-baseline gap-1.5">
                          <span className="text-sm font-medium text-gray-900">
                            {detail.userName}
                          </span>
                          <time
                            dateTime={detail.seenAt}
                            title={new Date(detail.seenAt).toLocaleString(
                              "de-DE",
                              { timeZone: "Europe/Berlin" },
                            )}
                            className="text-xs text-gray-400"
                          >
                            · {getRelativeTime(detail.seenAt)}
                          </time>
                        </div>
                        <Check className="text-moto-green h-4 w-4" />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Empty state */}
            {!isLoading &&
              viewDetails !== undefined &&
              confirmedUsers.length === 0 && (
                <p className="text-xs text-gray-400">
                  Noch keine Lesebestätigungen.
                </p>
              )}
          </div>
        </div>
      )}
    </div>
  );
}
