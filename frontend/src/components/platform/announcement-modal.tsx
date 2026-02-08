"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { Sparkles, Megaphone, Wrench } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { useAnnouncements } from "~/lib/hooks/use-announcements";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AnnouncementModal" });

// Contextual headers that create a positive, informative feeling
// title = small subtitle, subtitle = large main header
const DEFAULT_HEADER = {
  icon: Megaphone,
  title: "Neuigkeiten",
  subtitle: "Wichtige Informationen für Sie",
} as const;

const TYPE_HEADERS: Record<
  string,
  { icon: typeof Sparkles; title: string; subtitle: string }
> = {
  release: {
    icon: Sparkles,
    title: "Was ist neu?",
    subtitle: "Neue Funktionen und Verbesserungen",
  },
  announcement: DEFAULT_HEADER,
  maintenance: {
    icon: Wrench,
    title: "Wartungshinweis",
    subtitle: "Geplante Systemarbeiten",
  },
};

interface UnreadAnnouncement {
  id: number;
  title: string;
  content: string;
  type: string;
  severity: string;
  version?: string;
  published_at: string;
}

export function AnnouncementModal() {
  const { announcements, dismiss, isLoading, refresh } = useAnnouncements();
  const [currentIndex, setCurrentIndex] = useState(0);
  const [isVisible, setIsVisible] = useState(false);
  // Keep a stable snapshot of announcements when modal opens
  const [queuedAnnouncements, setQueuedAnnouncements] = useState<
    UnreadAnnouncement[]
  >([]);
  const dismissedIdsRef = useRef<Set<number>>(new Set());

  // Show modal when we have announcements - capture stable snapshot
  useEffect(() => {
    if (!isLoading && announcements.length > 0 && !isVisible) {
      // Filter out any we've already dismissed in this session
      const unprocessed = announcements.filter(
        (a) => !dismissedIdsRef.current.has(a.id),
      );
      if (unprocessed.length > 0) {
        setQueuedAnnouncements(unprocessed);
        setCurrentIndex(0);
        setIsVisible(true);
      }
    }
  }, [announcements, isLoading, isVisible]);

  const handleDismiss = useCallback(async () => {
    const current = queuedAnnouncements[currentIndex];
    if (!current) return;

    // Track this ID as dismissed locally
    dismissedIdsRef.current.add(current.id);

    // Send dismiss to backend (fire-and-forget, don't block UI)
    dismiss(current.id).catch((error) => {
      logger.error("announcement_dismiss_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
    });

    // Move to next or close
    if (currentIndex < queuedAnnouncements.length - 1) {
      setCurrentIndex((prev) => prev + 1);
    } else {
      setIsVisible(false);
      // Refresh data after all announcements processed
      void refresh();
    }
  }, [queuedAnnouncements, currentIndex, dismiss, refresh]);

  const current = queuedAnnouncements[currentIndex];
  if (!current || !isVisible) return null;

  const header = TYPE_HEADERS[current.type] ?? DEFAULT_HEADER;
  const Icon = header.icon;
  const totalCount = queuedAnnouncements.length;

  const footer = (
    <>
      <span className="flex-1 text-xs text-gray-500">
        {totalCount > 1 && `${currentIndex + 1} von ${totalCount}`}
      </span>
      <button
        type="button"
        onClick={() => void handleDismiss()}
        className="rounded-xl bg-gradient-to-br from-[#83CD2D] to-[#70b525] px-5 py-2.5 text-sm font-medium text-white shadow-md transition-all hover:scale-105 hover:shadow-lg active:scale-100"
      >
        {currentIndex < totalCount - 1 ? "Weiter" : "Verstanden"}
      </button>
    </>
  );

  return (
    <Modal
      isOpen={isVisible}
      onClose={() => void handleDismiss()}
      title=""
      footer={footer}
    >
      {/* Header section with icon and titles */}
      <div className="mb-5">
        <div className="flex items-start gap-3">
          <Icon className="mt-0.5 h-7 w-7 flex-shrink-0 text-[#83CD2D]" />
          <div>
            <h2 className="text-xl font-bold text-gray-900">
              {header.subtitle}
            </h2>
            <p className="mt-0.5 text-sm text-gray-500">
              {header.title}
              {current.version && (
                <span className="ml-2 text-gray-400">v{current.version}</span>
              )}
            </p>
          </div>
        </div>
      </div>

      {/* Divider */}
      <div className="mb-4 border-t border-gray-100" />

      {/* Announcement content */}
      <div>
        <h3 className="text-base font-semibold text-gray-900">
          {current.title}
        </h3>
        <p className="mt-2 text-sm whitespace-pre-wrap text-gray-600">
          {current.content}
        </p>
      </div>
    </Modal>
  );
}
