"use client";

import { notFound } from "next/navigation";
import { usePresenceMode } from "~/lib/tenant-context";

/**
 * BinaryModeGuard — triggers Next.js 404 when the tenant runs in binary mode.
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
 * Why a component, not a hook: calling `notFound()` from a hook forces every
 * page to import `notFound` itself and remember to gate render before data
 * loads. A wrapper component centralises the decision and keeps the gate
 * next to the route boundary.
 */
export function BinaryModeGuard({ children }: { children: React.ReactNode }) {
  const mode = usePresenceMode();
  if (mode === "binary") {
    notFound();
  }
  return <>{children}</>;
}
