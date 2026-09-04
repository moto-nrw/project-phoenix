"use client";

import Link from "next/link";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";
import {
  useContext,
  type ComponentProps,
  type FocusEvent,
  type PointerEvent,
} from "react";

type LinkProps = ComponentProps<typeof Link>;

export interface NavLinkProps extends Omit<LinkProps, "href" | "prefetch"> {
  readonly href: string;
}

/**
 * Link for the app shell (sidebar, bottom nav, breadcrumbs) that prefetches
 * on intent instead of on visibility (#2976).
 *
 * `next/link` prefetches every link the moment it scrolls into view. The shell
 * shows 20 to 30 of them on every page, and for the auth-bound tenant pages a
 * prefetch only yields the loading boundary, so each page view started and
 * aborted 10 to 28 RSC requests before the person had clicked anything.
 *
 * `prefetch={false}` switches that off, but in Next 16 it also removes the
 * hover prefetch. NavLink brings that part back by hand: the route is fetched
 * when the pointer enters the link (mouse hover, touch start) or when it gains
 * keyboard focus. The loading skeleton still appears instantly on click, and
 * nothing is requested for links nobody touches.
 *
 * Skipped, like `next/link` does itself: in development (the dev server would
 * compile every hovered route), for links that open a new tab, and for
 * anything that is not a same-origin path. The router comes from the same
 * context `next/link` reads, so the component also renders outside the app
 * router (tests, stories) and simply does not prefetch there.
 */
export function NavLink({
  href,
  target,
  onPointerEnter,
  onFocus,
  ...rest
}: NavLinkProps) {
  const router = useContext(AppRouterContext);
  const prefetchRouter =
    router !== null &&
    process.env.NODE_ENV !== "development" &&
    target !== "_blank" &&
    isInternalPath(href)
      ? router
      : null;

  const prefetch = () => {
    prefetchRouter?.prefetch(href);
  };

  return (
    <Link
      {...rest}
      href={href}
      target={target}
      prefetch={false}
      onPointerEnter={(event: PointerEvent<HTMLAnchorElement>) => {
        onPointerEnter?.(event);
        prefetch();
      }}
      onFocus={(event: FocusEvent<HTMLAnchorElement>) => {
        onFocus?.(event);
        prefetch();
      }}
    />
  );
}

function isInternalPath(href: string): boolean {
  return href.startsWith("/") && !href.startsWith("//");
}
