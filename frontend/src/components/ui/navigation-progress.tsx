"use client";

import { useLinkStatus } from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import {
  AppRouterContext,
  type AppRouterInstance,
} from "next/dist/shared/lib/app-router-context.shared-runtime";
import {
  createContext,
  Suspense,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useSyncExternalStore,
  type ReactNode,
} from "react";

/**
 * Fortschrittsanzeige für Seitenwechsel (#2828).
 *
 * Ohne Ladehülle bleibt beim Seitenwechsel die aktuelle Seite stehen, bis die
 * neue bereit ist. Das ist ruhig, aber ohne Rückmeldung: ein Klick, und für
 * einen Moment passiert sichtbar nichts. Der Balken schließt genau diese
 * Lücke — er ersetzt keine Seite, er liegt als 3 Pixel hohe Linie über dem
 * Kopfbereich.
 *
 * Woher der Zustand kommt: `useLinkStatus` meldet für jeden `NavLink`, ob
 * seine Navigation noch aussteht. Der Anbieter fängt außerdem `push` und
 * `replace` des App-Routers ab, weil einige Seiten per Schaltfläche wechseln.
 * Für vorgeladene Ziele (NavLink lädt bei Hover, Fokus oder Touch-Beginn vor)
 * überspringt Next den Link-Zustand ganz — dann erscheint der Balken gar nicht
 * erst, weil der Wechsel sofort passiert.
 *
 * Warum ein eigener Store statt `useState`: der Anbieter umschließt die
 * gesamte Hülle. Ein State-Wechsel dort würde Kopfzeile, Seitenleiste und
 * Inhalt bei jedem Klick neu rendern. So rendert nur der Balken selbst neu.
 */

interface NavigationProgressStore {
  readonly subscribe: (onChange: () => void) => () => void;
  readonly isPending: () => boolean;
  readonly startLink: () => void;
  readonly endLink: () => void;
  readonly startProgrammatic: (target: string | null) => number;
  readonly startHistory: () => number | null;
  readonly isHistoryPending: () => boolean;
  readonly completeProgrammatic: (currentUrl: string) => void;
  readonly completeHistory: (currentUrl: string) => void;
  readonly cancelProgrammatic: (id: number) => void;
}

const PROGRAMMATIC_NAVIGATION_TIMEOUT_MS = 10_000;

interface PendingProgrammaticNavigation {
  readonly id: number;
  readonly target: string | null;
  readonly supersededTargets: ReadonlySet<string>;
  readonly origin: string;
  readonly timeout: ReturnType<typeof setTimeout>;
}

function createStore(): NavigationProgressStore {
  let pendingLinks = 0;
  let pendingProgrammaticNavigation: PendingProgrammaticNavigation | null =
    null;
  let nextProgrammaticNavigationId = 0;
  const listeners = new Set<() => void>();
  const notify = () => {
    for (const listener of listeners) listener();
  };
  const isPending = () =>
    pendingLinks > 0 || pendingProgrammaticNavigation !== null;
  const update = (change: () => void) => {
    const wasPending = isPending();
    change();
    if (wasPending !== isPending()) notify();
  };
  const startProgrammatic = (target: string | null) => {
    const id = nextProgrammaticNavigationId + 1;
    nextProgrammaticNavigationId = id;
    update(() => {
      const previous = pendingProgrammaticNavigation;
      // Das App-Routing lässt nur das zuletzt gestartete Ziel gewinnen. Ein
      // verspätet eintreffendes, älteres Ziel ist kein Redirect des neuen
      // Wechsels und darf den Balken deshalb nicht vorzeitig beenden.
      const supersededTargets = new Set(previous?.supersededTargets);
      if (previous?.target !== null && previous?.target !== undefined) {
        supersededTargets.add(previous.target);
      }
      if (previous) clearTimeout(previous.timeout);
      const origin = currentUrl();
      pendingProgrammaticNavigation = {
        id,
        target,
        supersededTargets,
        origin,
        timeout: setTimeout(() => {
          if (pendingProgrammaticNavigation?.id !== id) return;
          update(() => {
            pendingProgrammaticNavigation = null;
          });
        }, PROGRAMMATIC_NAVIGATION_TIMEOUT_MS),
      };
    });
    return id;
  };
  return {
    subscribe: (onChange) => {
      listeners.add(onChange);
      return () => {
        listeners.delete(onChange);
      };
    },
    isPending,
    startLink: () => {
      update(() => {
        pendingLinks += 1;
      });
    },
    endLink: () => {
      update(() => {
        pendingLinks = Math.max(0, pendingLinks - 1);
      });
    },
    startProgrammatic,
    startHistory: () => {
      if (pendingProgrammaticNavigation?.target === null) return null;
      return startProgrammatic(null);
    },
    isHistoryPending: () => pendingProgrammaticNavigation?.target === null,
    completeProgrammatic: (url) => {
      const pending = pendingProgrammaticNavigation;
      if (!pending || pending.supersededTargets.has(url)) {
        return;
      }
      update(() => {
        clearTimeout(pending.timeout);
        pendingProgrammaticNavigation = null;
      });
    },
    completeHistory: (url) => {
      const pending = pendingProgrammaticNavigation;
      if (!pending || pending.target !== null || pending.origin !== url) {
        return;
      }
      update(() => {
        clearTimeout(pending.timeout);
        pendingProgrammaticNavigation = null;
      });
    },
    cancelProgrammatic: (id) => {
      if (pendingProgrammaticNavigation?.id !== id) return;
      update(() => {
        clearTimeout(pendingProgrammaticNavigation?.timeout);
        pendingProgrammaticNavigation = null;
      });
    },
  };
}

const NavigationProgressContext = createContext<NavigationProgressStore | null>(
  null,
);

/**
 * Umschließt die Hülle eines Portals. Außerhalb davon melden weder `NavLink`s
 * noch programmgesteuerte Wechsel etwas und der Balken erscheint nie —
 * Tests und Stories brauchen den Anbieter deshalb nicht.
 */
export function NavigationProgressProvider({
  children,
}: {
  readonly children: ReactNode;
}) {
  const store = useMemo(createStore, []);
  return (
    <NavigationProgressContext.Provider value={store}>
      <NavigationProgressRouter store={store}>
        {children}
      </NavigationProgressRouter>
    </NavigationProgressContext.Provider>
  );
}

function NavigationProgressRouter({
  children,
  store,
}: {
  readonly children: ReactNode;
  readonly store: NavigationProgressStore;
}) {
  const router: AppRouterInstance | null = useContext(AppRouterContext);
  // Alle Nachkommen erhalten einen gleichartigen Router. So deckt die
  // Fortschrittsanzeige bestehende router.push/replace/back/forward-Aufrufe
  // ab, ohne jede Schaltfläche auf einen eigenen Navigationshelfer umzustellen.
  const progressRouter = useMemo(() => {
    if (router === null) return null;

    return {
      ...router,
      push: (...args: Parameters<AppRouterInstance["push"]>) => {
        navigateTo(store, args[0], () => router.push(...args));
      },
      replace: (...args: Parameters<AppRouterInstance["replace"]>) => {
        navigateTo(store, args[0], () => router.replace(...args));
      },
      back: () => {
        navigateHistory(store, () => router.back());
      },
      forward: () => {
        navigateHistory(store, () => router.forward());
      },
    } satisfies AppRouterInstance;
  }, [router, store]);

  const content = (
    <>
      <Suspense fallback={null}>
        <NavigationProgressCompletion store={store} />
      </Suspense>
      {children}
    </>
  );

  if (progressRouter === null) return content;

  return (
    <AppRouterContext.Provider value={progressRouter}>
      {content}
    </AppRouterContext.Provider>
  );
}

function navigateTo(
  store: NavigationProgressStore,
  href: string,
  navigate: () => void,
) {
  const target = navigationTarget(href);
  if (target === null || target === currentUrl()) {
    navigate();
    return;
  }

  const id = store.startProgrammatic(target);
  try {
    navigate();
  } catch (error) {
    store.cancelProgrammatic(id);
    throw error;
  }
}

function navigateHistory(store: NavigationProgressStore, navigate: () => void) {
  const id = store.startHistory();
  try {
    navigate();
  } catch (error) {
    if (id !== null) store.cancelProgrammatic(id);
    throw error;
  }
}

function navigationTarget(href: string): string | null {
  try {
    const target = new URL(href, window.location.href);
    return `${target.pathname}${target.search}`;
  } catch {
    return null;
  }
}

function currentUrl() {
  return `${window.location.pathname}${window.location.search}`;
}

function NavigationProgressCompletion({
  store,
}: {
  readonly store: NavigationProgressStore;
}) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const search = searchParams?.toString() ?? "";
  const lastRouteUrl = useRef<string | null>(null);

  useEffect(() => {
    const url = currentUrl();
    store.completeProgrammatic(url);
    lastRouteUrl.current = url;
  }, [pathname, search, store]);

  useEffect(() => {
    const handleHistoryNavigation = () => {
      const url = currentUrl();
      if (store.isHistoryPending()) {
        store.completeHistory(url);
      } else if (
        lastRouteUrl.current !== null &&
        url !== lastRouteUrl.current
      ) {
        store.startHistory();
      }
    };
    window.addEventListener("popstate", handleHistoryNavigation);
    return () =>
      window.removeEventListener("popstate", handleHistoryNavigation);
  }, [store]);

  return null;
}

/**
 * Meldet den ausstehenden Seitenwechsel eines Links. Rendert nichts und muss
 * innerhalb eines `next/link` stehen — `NavLink` setzt sie dort ein.
 */
export function NavigationProgressReporter() {
  const store = useContext(NavigationProgressContext);
  const { pending } = useLinkStatus();

  useEffect(() => {
    if (!store || !pending) return;
    store.startLink();
    return store.endLink;
  }, [store, pending]);

  return null;
}

const NOT_PENDING = () => false;
const NO_SUBSCRIPTION = () => () => undefined;

/**
 * Der Balken selbst. Er liegt fest am oberen Rand, verschiebt nichts und
 * erscheint erst nach 150 ms — kurze Wechsel bleiben dadurch unsichtbar
 * ruhig, statt kurz aufzublitzen.
 */
export function NavigationProgressBar() {
  const store = useContext(NavigationProgressContext);
  const pending = useSyncExternalStore(
    store?.subscribe ?? NO_SUBSCRIPTION,
    store?.isPending ?? NOT_PENDING,
    NOT_PENDING,
  );

  return (
    <>
      {/* Dauerhaft im Baum, damit Screenreader die Änderung überhaupt
          vorlesen; ein erst beim Wechsel eingehängter Bereich wird nicht
          angesagt. */}
      <span role="status" aria-live="polite" className="sr-only">
        {pending ? "Seite wird geladen" : ""}
      </span>
      {pending ? (
        <div
          data-testid="navigation-progress"
          className="moto-nav-progress"
          aria-hidden="true"
        />
      ) : null}
    </>
  );
}
