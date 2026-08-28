"use client";

import { usePresenceMode } from "~/lib/tenant-context";
import { FeatureDisabledPage } from "./feature-disabled-page";

/**
 * BinaryModeGuard — renders the "Funktion ausgeschaltet" page when the tenant
 * runs in binary mode (#2624; previously notFound(), which wrongly showed
 * "Schule nicht gefunden").
 *
 * Use at the top of any page that depends on detailed-mode concepts (room
 * visits, activities, room supervision, room history). Binary-mode tenants
 * should never reach these pages via nav (the sidebar hides them), but
 * direct URL entry still has to be guarded for defense-in-depth.
 *
 * Usage:
 *   export default function RoomsPage() {
 *     return (
 *       <BinaryModeGuard>
 *         <RoomsPageContent />
 *       </BinaryModeGuard>
 *     );
 *   }
 *
 * Why a component, not a hook: a hook would force every page to remember to
 * gate render before data loads. A wrapper component centralises the decision
 * and keeps the gate next to the route boundary.
 */
export function BinaryModeGuard({ children }: { children: React.ReactNode }) {
  const mode = usePresenceMode();
  if (mode === "binary") {
    return <FeatureDisabledPage />;
  }
  return <>{children}</>;
}
