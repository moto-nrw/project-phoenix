"use client";

import Link from "~/components/ui/navigation-link";
import type { ComponentProps } from "react";

export type NavLinkProps = ComponentProps<typeof Link>;

/**
 * Link for the app shell (sidebar, bottom nav, breadcrumbs) that prefetches
 * on intent instead of on visibility (#2976, #4987).
 *
 * `next/link` prefetches every link the moment it scrolls into view. The shell
 * shows 20 to 30 of them on every page, so each page view started and aborted
 * 10 to 28 RSC requests before the person had clicked anything.
 *
 * `NavigationLink` keeps the viewport prefetch off and loads the full target
 * page, including its JS modules, once the pointer rests on the link, a touch
 * starts on it, or it gains keyboard focus. The shell uses the same link as
 * the page content; this name stays as the shell's entry point.
 */
export function NavLink(props: NavLinkProps) {
  return <Link {...props} />;
}
