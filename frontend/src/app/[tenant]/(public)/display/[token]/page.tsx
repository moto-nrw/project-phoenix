"use client";

import { use } from "react";
import { DisplayDashboardView } from "~/components/display/display-dashboard-view";

interface PageProps {
  params: Promise<{ tenant: string; token: string }>;
}

/**
 * Public info-point dashboard for large screens (issue #1325). No login —
 * the opaque token in the URL authenticates this specific display.
 */
export default function DisplayPage({ params }: PageProps) {
  const { token } = use(params);
  return <DisplayDashboardView token={token} />;
}
