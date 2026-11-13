"use client";

import { usePathname } from "next/navigation";
import { ContextHelpLink } from "~/components/help/context-help-link";
import { useBreadcrumb } from "~/lib/breadcrumb-context";
import {
  getHelpTopicForPath,
  getParentHelpTopicForPath,
} from "~/lib/help-topics";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import {
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { normalizeTenantPathname } from "~/lib/tenant-path";
import { SchoolSearchTrigger } from "~/components/search/school-search-trigger";

/** Mobile counterpart of the desktop header's help action. */
export function MobilePageHelpLink() {
  const auth = useShellAuthSafe();
  // Shared page primitives also render outside an authenticated portal.
  // moto schule already keeps its shell header visible on every width.
  if (auth?.mode !== "teacher" && auth?.mode !== "parent") return null;
  // Below lg the staff portal has no header: its search sits here, beside
  // the help (#6954).
  return (
    <>
      {auth.mode === "teacher" ? (
        <SchoolSearchTrigger placement="pageHead" />
      ) : null}
      <ResolvedMobilePageHelpLink mode={auth.mode} />
    </>
  );
}

function ResolvedMobilePageHelpLink({
  mode,
}: Readonly<{ mode: "teacher" | "parent" }>) {
  const rawPathname = usePathname();
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  const { breadcrumb } = useBreadcrumb();
  const pathname = normalizeTenantPathname(
    rawPathname,
    tenantSlug,
    routingMode,
  );
  const topic =
    mode === "parent"
      ? getParentHelpTopicForPath(pathname)
      : (breadcrumb.helpTopic ?? getHelpTopicForPath(pathname));

  return topic ? (
    <ContextHelpLink
      topic={topic}
      // Die Kopfkarte setzt die Bedienhöhe, die Fingerregel des Kits die
      // 44 px bei Fingerbedienung.
      containerClassName="inline-flex shrink-0 lg:hidden"
    />
  ) : null;
}
