"use client";

import { useEffect, useState } from "react";
import { setTenantAnalyticsPolicy } from "~/lib/posthog-client";

/** Each mounted school or unresolved enrollment target owns its own gate. */
export function useAnalyticsSchoolPolicy(
  tenantSlug: string,
  enabled: boolean,
): symbol {
  const [source] = useState(() => Symbol("school analytics policy"));
  useEffect(() => {
    setTenantAnalyticsPolicy(source, { tenantSlug, enabled });
  }, [source, tenantSlug, enabled]);
  useEffect(() => {
    return () => setTenantAnalyticsPolicy(source, null);
  }, [source]);
  return source;
}
