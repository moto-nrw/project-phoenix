"use client";

import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import { parentKeys } from "~/lib/swr/parent-keys";
import { type Child, listAnnouncements } from "~/lib/parent-api";
import {
  useParentChildren,
  useParentFeatures,
  PARENT_RESOURCE_OPTIONS,
} from "./use-parent-resources";

export function useParentNewsEnabled(
  enabled: boolean,
  children?: readonly Child[] | null,
): boolean {
  const { data: session } = useSession();
  const parentEnabled = enabled && session?.user.scope === "parent";
  const linked = useParentChildren(parentEnabled && children === undefined);
  const features = useParentFeatures(
    parentEnabled ? (children ?? linked.data) : undefined,
  );
  // Enrollment-only parents can receive news without a linked child.
  const feed = useSWRAuth(
    parentEnabled ? parentKeys.announcements() : null,
    listAnnouncements,
    PARENT_RESOURCE_OPTIONS,
  );
  return (
    (feed.data?.length ?? 0) > 0 ||
    Object.values(features.data ?? {}).some(
      (f) => f.parent_news_enabled === true,
    )
  );
}
