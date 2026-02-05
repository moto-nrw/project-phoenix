"use client";

import { useEffect, useState, useCallback } from "react";
import { Modal } from "~/components/ui/modal";
import {
  fetchUnreadAnnouncements,
  dismissAnnouncement,
} from "~/lib/platform-api";
import type { PlatformAnnouncement } from "~/lib/platform-api";

const LAST_CHECK_KEY = "announcements_last_checked";
const CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000; // 24 hours

const TYPE_BADGE_STYLES: Record<PlatformAnnouncement["type"], string> = {
  announcement: "bg-blue-100 text-blue-700",
  release: "bg-green-100 text-green-700",
  maintenance: "bg-yellow-100 text-yellow-800",
};

const TYPE_BADGE_LABELS: Record<PlatformAnnouncement["type"], string> = {
  announcement: "Ankündigung",
  release: "Release",
  maintenance: "Wartung",
};

export function AnnouncementModal() {
  const [announcements, setAnnouncements] = useState<PlatformAnnouncement[]>(
    [],
  );
  const [currentIndex, setCurrentIndex] = useState(0);
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    const checkAnnouncements = async () => {
      // Throttle to once per day
      const lastCheck = localStorage.getItem(LAST_CHECK_KEY);
      if (lastCheck) {
        const elapsed = Date.now() - Number.parseInt(lastCheck, 10);
        if (elapsed < CHECK_INTERVAL_MS) return;
      }

      try {
        const unread = await fetchUnreadAnnouncements();
        if (unread.length > 0) {
          setAnnouncements(unread);
          setCurrentIndex(0);
          setIsOpen(true);
        }
        localStorage.setItem(LAST_CHECK_KEY, Date.now().toString());
      } catch (error) {
        console.error("Failed to fetch announcements:", error);
      }
    };

    void checkAnnouncements();
  }, []);

  const handleDismiss = useCallback(async () => {
    const current = announcements[currentIndex];
    if (!current) return;

    try {
      await dismissAnnouncement(current.id);
    } catch (error) {
      console.error("Failed to dismiss announcement:", error);
    }

    if (currentIndex < announcements.length - 1) {
      setCurrentIndex((prev) => prev + 1);
    } else {
      setIsOpen(false);
    }
  }, [announcements, currentIndex]);

  const current = announcements[currentIndex];
  if (!current) return null;

  return (
    <Modal
      isOpen={isOpen}
      onClose={() => setIsOpen(false)}
      title=""
      footer={
        <div className="flex w-full items-center justify-between">
          <span className="text-sm text-gray-500">
            {currentIndex + 1} von {announcements.length}
          </span>
          <button
            type="button"
            onClick={() => void handleDismiss()}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
          >
            {currentIndex < announcements.length - 1 ? "Weiter" : "Verstanden"}
          </button>
        </div>
      }
    >
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <span
            className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${TYPE_BADGE_STYLES[current.type]}`}
          >
            {TYPE_BADGE_LABELS[current.type]}
          </span>
          {current.version && (
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
              v{current.version}
            </span>
          )}
        </div>
        <h3 className="text-lg font-semibold text-gray-900">{current.title}</h3>
        <p className="text-sm whitespace-pre-wrap text-gray-600">
          {current.content}
        </p>
      </div>
    </Modal>
  );
}
