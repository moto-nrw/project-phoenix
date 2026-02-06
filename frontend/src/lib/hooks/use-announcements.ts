"use client";

import { useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import useSWR from "swr";
import { authFetch } from "~/lib/api-helpers";

interface UnreadAnnouncement {
  id: number;
  title: string;
  content: string;
  type: string;
  severity: string;
  version?: string;
  published_at: string;
}

interface UnreadResponse {
  data: UnreadAnnouncement[];
}

async function fetchUnread(): Promise<UnreadAnnouncement[]> {
  const response = await authFetch<UnreadResponse>(
    "/api/platform/announcements/unread",
  );
  return response.data ?? [];
}

async function markDismissed(id: number): Promise<void> {
  await authFetch(`/api/platform/announcements/${id}/dismiss`, {
    method: "POST",
  });
}

export function useAnnouncements() {
  const pathname = usePathname();
  const previousPathname = useRef(pathname);

  const { data, mutate, isLoading } = useSWR(
    "user-announcements-unread",
    fetchUnread,
    {
      refreshInterval: 60000, // Poll every 60s
      revalidateOnFocus: false, // Don't revalidate on window focus to avoid showing dismissed items again
      revalidateOnMount: true, // Always fetch fresh data on component mount
      dedupingInterval: 5000, // Prevent rapid refetches
    },
  );

  // Revalidate on route change (since component stays mounted in layout)
  useEffect(() => {
    if (previousPathname.current !== pathname) {
      previousPathname.current = pathname;
      void mutate();
    }
  }, [pathname, mutate]);

  const dismiss = async (id: number) => {
    // Just send to backend - don't mutate local state
    // The modal manages its own queue and will refresh after closing
    await markDismissed(id);
  };

  return {
    announcements: data ?? [],
    unreadCount: data?.length ?? 0,
    isLoading,
    dismiss,
    refresh: () => mutate(),
  };
}
