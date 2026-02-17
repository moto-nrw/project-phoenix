"use client";

import { Loading } from "~/components/ui/loading";

/**
 * Content-area-only loading skeleton for the (protected) route group.
 * Renders inline within the persistent shell (Header/Sidebar stay mounted).
 * Uses fullPage=false so there's no fixed overlay or z-50.
 */
export default function ProtectedLoadingPage() {
  return <Loading message="Laden..." fullPage={false} />;
}
