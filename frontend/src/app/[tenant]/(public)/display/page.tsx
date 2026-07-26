"use client";

import { useEffect, useState } from "react";
import {
  DisplayDashboardView,
  DisplayInvalidScreen,
} from "~/components/display/display-dashboard-view";

/**
 * Public info-point dashboard for large screens (issue #1325). No login —
 * the opaque token in the URL fragment (/display#<token>) authenticates this
 * specific display. The fragment is never sent to any server, so the token
 * cannot leak into request logs; the API poll carries it in a header instead.
 */
export default function DisplayPage() {
  // null = fragment not read yet (SSR / first paint); "" = link has no token.
  const [token, setToken] = useState<string | null>(null);

  useEffect(() => {
    setToken(decodeURIComponent(window.location.hash.replace(/^#/, "")));
  }, []);

  if (token === null) {
    return null;
  }
  if (token === "") {
    return <DisplayInvalidScreen />;
  }
  return <DisplayDashboardView token={token} />;
}
