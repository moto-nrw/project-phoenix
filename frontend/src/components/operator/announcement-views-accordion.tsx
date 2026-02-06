"use client";

import { useState, useCallback, useRef, useMemo } from "react";
import { ChevronDown, Check } from "lucide-react";
import { operatorAnnouncementsService } from "~/lib/operator/announcements-api";
import type { AnnouncementViewDetail } from "~/lib/operator/announcements-helpers";

function formatUnit(value: number, singular: string, plural: string): string {
  return `vor ${value} ${value === 1 ? singular : plural}`;
}

function getRelativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 1) return "gerade eben";
  if (minutes < 60) return formatUnit(minutes, "Minute", "Minuten");

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return formatUnit(hours, "Stunde", "Stunden");

  const days = Math.floor(hours / 24);
  if (days < 7) return formatUnit(days, "Tag", "Tagen");

  const weeks = Math.floor(days / 7);
  if (weeks < 5) return formatUnit(weeks, "Woche", "Wochen");

  const months = Math.floor(days / 30);
  if (months < 12) return formatUnit(months, "Monat", "Monaten");

  const years = Math.floor(days / 365);
  return formatUnit(years, "Jahr", "Jahren");
}

function getInitial(name: string): string {
  return name.charAt(0).toUpperCase() || "?";
}

interface AnnouncementViewsAccordionProps {
  readonly announcementId: string;
  readonly dismissedCount: number;
}

export function AnnouncementViewsAccordion({
  announcementId,
  dismissedCount,
}: AnnouncementViewsAccordionProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [viewDetails, setViewDetails] = useState<AnnouncementViewDetail[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const loadedRef = useRef(false);

  // Only show users who confirmed (clicked "Verstanden")
  const confirmedUsers = useMemo(
    () => viewDetails.filter((d) => d.dismissed),
    [viewDetails],
  );

  const loadViewDetails = useCallback(async () => {
    if (loadedRef.current) return;
    setIsLoading(true);
    setError(null);
    try {
      const details =
        await operatorAnnouncementsService.fetchViewDetails(announcementId);
      setViewDetails(details);
      loadedRef.current = true;
    } catch {
      setError("Ansichten konnten nicht geladen werden.");
    } finally {
      setIsLoading(false);
    }
  }, [announcementId]);

  const handleToggle = useCallback(() => {
    const opening = !isOpen;
    setIsOpen(opening);
    if (opening) {
      void loadViewDetails();
    }
  }, [isOpen, loadViewDetails]);

  // Only show if there are any confirmations
  if (dismissedCount === 0) {
    return null;
  }

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
        onClick={handleToggle}
        className="flex w-full items-center justify-between py-3 text-sm text-gray-600 transition-colors hover:text-gray-900"
      >
        <span className="flex items-center gap-1.5 font-medium">
          Lesebestätigungen
          <span className="text-xs font-normal text-gray-400">
            ({dismissedCount})
          </span>
        </span>
        <ChevronDown
          className={`h-4 w-4 transition-transform duration-200 ${isOpen ? "rotate-180" : ""}`}
        />
      </button>

      {/* Accordion body */}
      <div
        className={`grid transition-[grid-template-rows] duration-200 ${isOpen ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
      >
        <div className="overflow-hidden">
          <div className="pb-3">
            {/* Loading state */}
            {isLoading && (
              <p className="py-2 text-xs text-gray-400">Laden...</p>
            )}

            {/* Error */}
            {error && <p className="mb-2 text-xs text-red-500">{error}</p>}

            {/* Confirmed users list */}
            {!isLoading && confirmedUsers.length > 0 && (
              <div className="space-y-0 divide-y divide-gray-100">
                {confirmedUsers.map((detail) => (
                  <div
                    key={detail.userId}
                    className="flex items-center gap-2.5 py-2.5 first:pt-0 last:pb-0"
                  >
                    {/* Initials avatar */}
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-green-100 text-xs font-medium text-green-600">
                      {getInitial(detail.userName)}
                    </div>

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
                            )}
                            className="text-xs text-gray-400"
                          >
                            · {getRelativeTime(detail.seenAt)}
                          </time>
                        </div>
                        <Check className="h-4 w-4 text-green-500" />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Empty state */}
            {!isLoading && loadedRef.current && confirmedUsers.length === 0 && (
              <p className="text-xs text-gray-400">
                Noch keine Lesebestätigungen.
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
