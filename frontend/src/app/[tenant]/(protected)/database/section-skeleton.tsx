"use client";

import { useSelectedLayoutSegments } from "next/navigation";
import { MasterDetailSkeleton } from "~/components/database/master-detail-skeleton";
import { Loading } from "~/components/ui/loading";
import { DatabaseIndexSkeleton } from "./page-skeleton";

const MASTER_DETAIL_SEGMENTS = new Set([
  "activities",
  "devices",
  "groups",
  "permissions",
  "personal",
  "roles",
  "rooms",
  "students",
]);

export function DatabaseSectionSkeleton() {
  const segments = useSelectedLayoutSegments();

  if (segments.length === 0) return <DatabaseIndexSkeleton />;
  if (segments.length === 1 && MASTER_DETAIL_SEGMENTS.has(segments[0] ?? "")) {
    return <MasterDetailSkeleton />;
  }

  return <Loading message="Datenverwaltung wird geladen…" fullPage={false} />;
}
