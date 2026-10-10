"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { parentPath } from "~/lib/parent-url";

const POLL_MS = 200;
const MISSING_AFTER_MS = 4000;
const ADVANCE_DELAY_MS = 250;
const MIN_VISIBLE_PX = 8;

export type ParentFirstStepKey =
  "start" | "childData" | "messagesNews" | "installApp" | "notifications";

interface ParentTourStop {
  readonly targets: string | readonly string[];
  readonly title: string;
  readonly text: string;
  readonly advance: "click" | "next";
  /** Das Ziel steht in der Navigation und darf auf jeder Seite gesucht werden. */
  readonly nav?: true;
  /** Diese Station entfällt, wenn die Zielseite schon geöffnet ist. */
  readonly skipOnPath?: string;
  readonly skipOnPathPrefix?: string;
  /** Diese Station entfällt, wenn ein späteres Ziel schon sichtbar ist. */
  readonly skipWhenVisible?: string;
  /** Zeigt die Station ohne Ziel, sobald feststeht, dass die Funktion fehlt. */
  readonly missingWhenPresent?: string;
  readonly missingText?: string;
  /** Seite, auf der das Ziel gesucht wird. */
  readonly path?: string;
  readonly pathPrefix?: string;
}

export interface ParentTourDefinition {
  readonly path: string;
  readonly stops: readonly ParentTourStop[];
}

export interface ParentTourCopy {
  readonly start: {
    title: string;
    navigation: string;
    todoTitle: string;
    todo: string;
    childTitle: string;
    child: string;
  };
  readonly childData: {
    title: string;
    navigation: string;
    chooseTitle: string;
    choose: string;
    tabTitle: string;
    tab: string;
    detailsTitle: string;
    details: string;
    careTabTitle: string;
    careTab: string;
    careTimesTitle: string;
    careTimes: string;
    departureTitle: string;
    departure: string;
    contactsTabTitle: string;
    contactsTab: string;
    guardiansTitle: string;
    guardians: string;
  };
  readonly messagesNews: {
    title: string;
    messagesNavigation: string;
    messagesTitle: string;
    messages: string;
    moreTitle: string;
    more: string;
    newsNavigation: string;
    newsTitle: string;
    news: string;
  };
  readonly installApp: {
    title: string;
    settingsNavigation: string;
    moreTitle: string;
    more: string;
    guideTitle: string;
    guide: string;
  };
  readonly notifications: {
    title: string;
    settingsNavigation: string;
    moreTitle: string;
    more: string;
    topicsTitle: string;
    topics: string;
    deviceTitle: string;
    device: string;
    deviceUnavailable: string;
  };
}

function settingsNavigationStops(
  title: string,
  navigation: string,
  moreTitle: string,
  more: string,
): ParentTourStop[] {
  return [
    {
      targets: [
        '[data-parent-nav-item="settings"]',
        '[data-parent-nav-item="more"]',
      ],
      title: moreTitle,
      text: more,
      advance: "click",
      nav: true,
      skipOnPath: "/parents/settings",
    },
    {
      targets: '[data-parent-nav-item="settings"]',
      title,
      text: navigation,
      advance: "click",
      nav: true,
      skipOnPath: "/parents/settings",
    },
  ];
}

export function buildParentTours(
  copy: ParentTourCopy,
  options: Readonly<{ hasChild: boolean; newsEnabled: boolean }>,
): Readonly<Record<ParentFirstStepKey, ParentTourDefinition>> {
  const startStops: ParentTourStop[] = [
    {
      targets: '[data-parent-nav-item="start"]',
      title: copy.start.title,
      text: copy.start.navigation,
      advance: "click",
      nav: true,
      skipOnPath: "/parents",
    },
    {
      targets: '[data-parent-tour="start-todo"]',
      title: copy.start.todoTitle,
      text: copy.start.todo,
      advance: "next",
      path: "/parents",
    },
  ];
  if (options.hasChild) {
    startStops.push({
      targets: '[data-parent-tour="start-child"]',
      title: copy.start.childTitle,
      text: copy.start.child,
      advance: "next",
      path: "/parents",
    });
  }

  const messagesStops: ParentTourStop[] = [
    {
      targets: '[data-parent-nav-item="messages"]',
      title: copy.messagesNews.title,
      text: copy.messagesNews.messagesNavigation,
      advance: "click",
      nav: true,
      skipOnPathPrefix: "/parents/messages",
    },
    {
      targets: '[data-parent-tour="messages"]',
      title: copy.messagesNews.messagesTitle,
      text: copy.messagesNews.messages,
      advance: "next",
      pathPrefix: "/parents/messages",
    },
  ];
  if (options.newsEnabled) {
    messagesStops.push(
      {
        targets: [
          '[data-parent-nav-item="news"]',
          '[data-parent-nav-item="more"]',
        ],
        title: copy.messagesNews.moreTitle,
        text: copy.messagesNews.more,
        advance: "click",
        nav: true,
        skipOnPathPrefix: "/parents/news",
      },
      {
        targets: '[data-parent-nav-item="news"]',
        title: copy.messagesNews.title,
        text: copy.messagesNews.newsNavigation,
        advance: "click",
        nav: true,
        skipOnPathPrefix: "/parents/news",
      },
      {
        targets: '[data-parent-tour="news"]',
        title: copy.messagesNews.newsTitle,
        text: copy.messagesNews.news,
        advance: "next",
        pathPrefix: "/parents/news",
      },
    );
  }

  return {
    start: { path: "/parents", stops: startStops },
    childData: {
      path: "/parents/children",
      stops: [
        {
          targets: '[data-parent-nav-item="children"]',
          title: copy.childData.title,
          text: copy.childData.navigation,
          advance: "click",
          nav: true,
          skipOnPathPrefix: "/parents/children",
        },
        {
          targets: '[data-parent-tour="child-switcher-item"]',
          title: copy.childData.chooseTitle,
          text: copy.childData.choose,
          advance: "click",
          pathPrefix: "/parents/children",
          skipWhenVisible: '[data-parent-tour="child-area-tabs"]',
        },
        {
          targets: '[data-parent-tour="child-data-tab"]',
          title: copy.childData.tabTitle,
          text: copy.childData.tab,
          advance: "click",
          pathPrefix: "/parents/children",
          skipWhenVisible: '[data-parent-tour="child-data"]',
        },
        {
          targets: '[data-parent-tour="child-data"]',
          title: copy.childData.detailsTitle,
          text: copy.childData.details,
          advance: "next",
          pathPrefix: "/parents/children",
        },
        {
          targets: '[data-parent-tour="child-care-tab"]',
          title: copy.childData.careTabTitle,
          text: copy.childData.careTab,
          advance: "click",
          pathPrefix: "/parents/children",
          skipWhenVisible: '[data-parent-tour="child-care"]',
        },
        {
          targets: 'section:has([data-parent-tour="child-care-times"])',
          title: copy.childData.careTimesTitle,
          text: copy.childData.careTimes,
          advance: "next",
          pathPrefix: "/parents/children",
        },
        {
          targets: 'section:has([data-parent-tour="child-departure"])',
          title: copy.childData.departureTitle,
          text: copy.childData.departure,
          advance: "next",
          pathPrefix: "/parents/children",
        },
        {
          targets: '[data-parent-tour="child-contacts-tab"]',
          title: copy.childData.contactsTabTitle,
          text: copy.childData.contactsTab,
          advance: "click",
          pathPrefix: "/parents/children",
          skipWhenVisible: '[data-parent-tour="child-contacts"]',
        },
        {
          targets: 'section:has([data-parent-tour="child-guardians"])',
          title: copy.childData.guardiansTitle,
          text: copy.childData.guardians,
          advance: "next",
          pathPrefix: "/parents/children",
        },
      ],
    },
    messagesNews: {
      path: "/parents/messages",
      stops: messagesStops,
    },
    installApp: {
      path: "/parents/settings",
      stops: [
        ...settingsNavigationStops(
          copy.installApp.title,
          copy.installApp.settingsNavigation,
          copy.installApp.moreTitle,
          copy.installApp.more,
        ),
        {
          targets: [
            "[data-parent-tour-install-app]",
            '[data-parent-tour="notification-device"]',
          ],
          title: copy.installApp.guideTitle,
          text: copy.installApp.guide,
          advance: "next",
          path: "/parents/settings",
        },
      ],
    },
    notifications: {
      path: "/parents/settings",
      stops: [
        ...settingsNavigationStops(
          copy.notifications.title,
          copy.notifications.settingsNavigation,
          copy.notifications.moreTitle,
          copy.notifications.more,
        ),
        {
          targets: '[data-parent-tour="notification-topics"]',
          title: copy.notifications.topicsTitle,
          text: copy.notifications.topics,
          advance: "next",
          path: "/parents/settings",
        },
        {
          targets: '[data-parent-tour="notification-device"]',
          title: copy.notifications.deviceTitle,
          text: copy.notifications.device,
          advance: "next",
          path: "/parents/settings",
          missingWhenPresent:
            '[data-parent-tour="notification-device-unavailable"]',
          missingText: copy.notifications.deviceUnavailable,
        },
      ],
    },
  };
}

function isShown(element: Element): boolean {
  if (element.closest("[inert]")) return false;
  if (element.getClientRects().length === 0) return false;
  const rect = element.getBoundingClientRect();
  let top = rect.top;
  let bottom = rect.bottom;
  let left = rect.left;
  let right = rect.right;
  for (
    let parent = element.parentElement;
    parent && parent !== document.body;
    parent = parent.parentElement
  ) {
    const style = window.getComputedStyle(parent);
    if (style.visibility === "hidden" || style.display === "none") return false;
    const clips = [style.overflowX, style.overflowY].some(
      (value) => value === "hidden" || value === "clip",
    );
    if (!clips) continue;
    const clip = parent.getBoundingClientRect();
    top = Math.max(top, clip.top);
    bottom = Math.min(bottom, clip.bottom);
    left = Math.max(left, clip.left);
    right = Math.min(right, clip.right);
  }
  return bottom - top >= MIN_VISIBLE_PX && right - left >= MIN_VISIBLE_PX;
}

function findParentTourTarget(
  selectors: string | readonly string[],
): Element | null {
  const candidates = typeof selectors === "string" ? [selectors] : selectors;
  for (const selector of candidates) {
    for (const element of Array.from(document.querySelectorAll(selector))) {
      if (isShown(element)) return element;
    }
  }
  return null;
}

function isPath(pathname: string, path: string): boolean {
  return pathname === parentPath(path);
}

function isPathPrefix(pathname: string, prefix: string): boolean {
  const publicPrefix = parentPath(prefix);
  return (
    pathname === publicPrefix ||
    (publicPrefix === "/"
      ? pathname.startsWith("/")
      : pathname.startsWith(`${publicPrefix}/`))
  );
}

function shouldSkipStop(stop: ParentTourStop, pathname: string): boolean {
  return Boolean(
    (stop.skipOnPath && isPath(pathname, stop.skipOnPath)) ||
    (stop.skipOnPathPrefix && isPathPrefix(pathname, stop.skipOnPathPrefix)) ||
    (stop.skipWhenVisible && findParentTourTarget(stop.skipWhenVisible)),
  );
}

function sharesSkipPath(
  current: ParentTourStop,
  next: ParentTourStop,
): boolean {
  return Boolean(
    (current.skipOnPath && current.skipOnPath === next.skipOnPath) ||
    (current.skipOnPathPrefix &&
      current.skipOnPathPrefix === next.skipOnPathPrefix),
  );
}

function previousVisitedStop(
  visited: readonly number[],
  index: number,
): { index: number; visited: readonly number[] } | null {
  const currentPosition = visited.lastIndexOf(index);
  const previousPosition =
    currentPosition === -1 ? visited.length - 1 : currentPosition - 1;
  const previousIndex = visited[previousPosition];
  if (previousIndex === undefined) return null;
  return {
    index: previousIndex,
    visited: visited.slice(0, previousPosition + 1),
  };
}

interface ActiveTour {
  readonly key: ParentFirstStepKey;
  readonly stop: ParentTourStop;
  readonly index: number;
  readonly total: number;
  readonly target: Element | null;
  readonly missing: boolean;
  readonly canGoBack: boolean;
}

export function useParentFirstStepsTour(
  definitions: Readonly<Record<ParentFirstStepKey, ParentTourDefinition>>,
  onFinished: (key: ParentFirstStepKey) => void,
): {
  active: ActiveTour | null;
  start: (key: ParentFirstStepKey) => void;
  next: () => void;
  back: () => void;
  stop: () => void;
} {
  const pathname = usePathname();
  const router = useRouter();
  const [tour, setTour] = useState<{
    key: ParentFirstStepKey;
    index: number;
    visited: readonly number[];
  } | null>(null);
  const [search, setSearch] = useState<{
    stop: ParentTourStop;
    target: Element | null;
    missing: boolean;
  } | null>(null);

  const definition = tour ? definitions[tour.key] : null;
  const stop = definition && tour ? definition.stops[tour.index] : undefined;
  const current = search?.stop === stop ? search : null;

  const finish = useCallback(() => {
    if (!tour) return;
    const key = tour.key;
    setTour(null);
    setSearch(null);
    onFinished(key);
  }, [onFinished, tour]);

  const next = useCallback(() => {
    if (!tour) return;
    const stops = definitions[tour.key].stops;
    if (tour.index + 1 >= stops.length) {
      finish();
      return;
    }
    setTour({ ...tour, index: tour.index + 1 });
    setSearch(null);
  }, [definitions, finish, tour]);

  const back = useCallback(() => {
    if (!tour || tour.index === 0) return;
    const previous = previousVisitedStop(tour.visited, tour.index);
    if (!previous) return;
    setTour({ ...tour, ...previous });
    setSearch(null);
  }, [tour]);

  const start = useCallback((key: ParentFirstStepKey) => {
    setSearch(null);
    setTour({ key, index: 0, visited: [] });
  }, []);

  const end = useCallback(() => {
    setTour(null);
    setSearch(null);
  }, []);

  useEffect(() => {
    if (!tour || !stop) return;
    const previousStop = definition?.stops[tour.index - 1];
    // Der gemeinsame Pfad kann das Ergebnis der vorherigen Station sein. In
    // diesem Fall bleibt die nächste Navigationserklärung sichtbar.
    const followsVisitedStopOnSamePath =
      previousStop !== undefined &&
      tour.visited.includes(tour.index - 1) &&
      sharesSkipPath(previousStop, stop);
    if (
      !tour.visited.includes(tour.index) &&
      !followsVisitedStopOnSamePath &&
      shouldSkipStop(stop, pathname)
    ) {
      next();
    }
  }, [definition, next, pathname, stop, tour]);

  useEffect(() => {
    if (!tour || !stop) return undefined;
    const onExpectedPage =
      stop.nav === true ||
      (stop.path === undefined && stop.pathPrefix === undefined) ||
      (stop.path !== undefined && isPath(pathname, stop.path)) ||
      (stop.pathPrefix !== undefined &&
        isPathPrefix(pathname, stop.pathPrefix));
    if (!onExpectedPage) return undefined;

    let polls = 0;
    let advanced = false;
    const rememberCurrentStop = () => {
      setTour((currentTour) => {
        if (
          !currentTour ||
          currentTour.key !== tour.key ||
          currentTour.index !== tour.index ||
          currentTour.visited.at(-1) === currentTour.index
        ) {
          return currentTour;
        }
        return {
          ...currentTour,
          visited: [...currentTour.visited, currentTour.index],
        };
      });
    };
    const check = () => {
      if (advanced) return;
      if (
        stop.missingWhenPresent &&
        document.querySelector(stop.missingWhenPresent)
      ) {
        advanced = true;
        rememberCurrentStop();
        setSearch({ stop, target: null, missing: true });
        return;
      }
      if (
        !tour.visited.includes(tour.index) &&
        stop.skipWhenVisible &&
        findParentTourTarget(stop.skipWhenVisible)
      ) {
        advanced = true;
        next();
        return;
      }
      const target = findParentTourTarget(stop.targets);
      if (target) {
        rememberCurrentStop();
        setSearch({ stop, target, missing: false });
        return;
      }
      polls += 1;
      if (polls * POLL_MS > MISSING_AFTER_MS) {
        rememberCurrentStop();
        setSearch({ stop, target: null, missing: true });
      }
    };
    check();
    const timer = window.setInterval(check, POLL_MS);
    return () => window.clearInterval(timer);
  }, [next, pathname, stop, tour]);

  const target = current?.target ?? null;
  const previous = tour ? previousVisitedStop(tour.visited, tour.index) : null;
  useEffect(() => {
    if (!stop || !target || stop.advance !== "click") return undefined;
    const onClick = (event: MouseEvent) => {
      if (!(event.target instanceof Node) || !target.contains(event.target)) {
        return;
      }
      window.setTimeout(next, ADVANCE_DELAY_MS);
    };
    document.addEventListener("click", onClick, true);
    return () => document.removeEventListener("click", onClick, true);
  }, [next, stop, target]);

  // Falls eine Navigationsstelle auf einem ungewöhnlichen Bildschirm fehlt,
  // führt „Weiter“ trotzdem auf die richtige Seite der Tour.
  const continueFromMissing = useCallback(() => {
    if (!tour || !definition) return;
    router.push(parentPath(definition.path));
    next();
  }, [definition, next, router, tour]);

  return useMemo(
    () => ({
      active:
        tour && stop && definition
          ? {
              key: tour.key,
              stop,
              index: tour.index,
              total: definition.stops.length,
              target,
              missing: current?.missing ?? false,
              canGoBack: previous !== null,
            }
          : null,
      start,
      next: current?.missing ? continueFromMissing : next,
      back,
      stop: end,
    }),
    [
      back,
      continueFromMissing,
      current?.missing,
      definition,
      end,
      next,
      previous,
      start,
      stop,
      target,
      tour,
    ],
  );
}
