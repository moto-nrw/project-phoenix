"use client";

import { useSession } from "next-auth/react";
import { fetchUnreadCount } from "~/lib/suggestions-api";
import { useUnreadCount } from "./use-unread-count";

export function useSuggestionsUnread() {
  const { status } = useSession();
  return useUnreadCount({
    enabled: status === "authenticated",
    fetcher: fetchUnreadCount,
    cacheKey: "suggestions_unread_count",
    eventNames: ["suggestions-unread-refresh"],
  });
}
