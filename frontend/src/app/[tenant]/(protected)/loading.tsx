"use client";

import { ListPageSkeleton } from "~/components/ui/page-skeletons";

/**
 * Content-area-only loading skeleton for the (protected) route group.
 * Renders inline within the persistent shell (Header/Sidebar stay mounted).
 * Generic list-page silhouette (header + table) since the actual route
 * being navigated to is not known here.
 */
export default function ProtectedLoadingPage() {
  return <ListPageSkeleton label="Laden..." chips={3} rows={7} columns={5} />;
}
