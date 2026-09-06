"use client";

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
 * Woher der Zustand kommt: `NavigationLink` meldet jeden tatsächlich
 * gestarteten clientseitigen Link-Wechsel vor dem Router-Dispatch. Der
 * Anbieter fängt außerdem `push` und `replace` des App-Routers ab, weil einige
 * Seiten per Schaltfläche wechseln. Für vorgeladene Ziele (NavLink lädt bei
 * Hover, Fokus oder Touch-Beginn vor) ist der Wechsel oft sofort fertig und
 * der Balken bleibt unsichtbar.
 *
 * Warum ein eigener Store statt `useState`: der Anbieter umschließt die
 * gesamte Hülle. Ein State-Wechsel dort würde Kopfzeile, Seitenleiste und
 * Inhalt bei jedem Klick neu rendern. So rendert nur der Balken selbst neu.
 */

export interface NavigationProgressStore {
  readonly subscribe: (onChange: () => void) => () => void;
  readonly isPending: () => boolean;
  readonly isFallbackSuppressed: () => boolean;
  readonly startNavigation: (target: string) => number;
  readonly startLinkNavigation: (target: string) => number;
  readonly startHistory: (target: string) => void;
  readonly completeNavigation: (currentUrl: string) => void;
  readonly cancelNavigation: (id: number) => void;
}

const NAVIGATION_TIMEOUT_MS = 10_000;

interface PendingNavigation {
  readonly id: number;
  readonly target: string;
  readonly timeout: ReturnType<typeof setTimeout>;
}

function createStore(): NavigationProgressStore {
  let pendingNavigations: PendingNavigation[] = [];
  let pendingHistoryNavigation: PendingNavigation | null = null;
  let pendingLinkNavigation: Pick<PendingNavigation, "id" | "target"> | null =
    null;
  let nextNavigationId = 0;
  const listeners = new Set<() => void>();
  const notify = () => {
    for (const listener of listeners) listener();
  };
  const isPending = () =>
    pendingNavigations.length > 0 || pendingHistoryNavigation !== null;
  const isFallbackSuppressed = isPending;
  const update = (change: () => void) => {
    const wasPending = isPending();
    change();
    if (wasPending !== isPending()) notify();
  };
  const clearNavigations = () => {
    for (const navigation of pendingNavigations) {
      clearTimeout(navigation.timeout);
    }
    pendingNavigations = [];
  };
  const clearHistoryNavigation = () => {
    if (pendingHistoryNavigation) {
      clearTimeout(pendingHistoryNavigation.timeout);
      pendingHistoryNavigation = null;
    }
  };
  const startNavigation = (target: string) => {
    if (pendingLinkNavigation?.target === target) {
      return pendingLinkNavigation.id;
    }

    const id = nextNavigationId + 1;
    nextNavigationId = id;
    update(() => {
      clearHistoryNavigation();
      // Der App-Router arbeitet Seitenwechsel nacheinander ab. Dasselbe Ziel
      // kann deshalb mehrmals ausstehen und braucht je Wechsel eine eigene
      // Kennung.
      const navigation: PendingNavigation = {
        id,
        target,
        timeout: setTimeout(() => {
          update(() => {
            const index = pendingNavigations.findIndex(
              (pending) => pending.id === id,
            );
            if (index !== -1) pendingNavigations.splice(index, 1);
          });
        }, NAVIGATION_TIMEOUT_MS),
      };
      pendingNavigations.push(navigation);
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
    isFallbackSuppressed,
    startNavigation,
    startLinkNavigation: (target) => {
      const id = startNavigation(target);
      pendingLinkNavigation = { id, target };
      queueMicrotask(() => {
        if (pendingLinkNavigation?.id === id) {
          pendingLinkNavigation = null;
        }
      });
      return id;
    },
    startHistory: (target) => {
      // Verlaufwechsel haben ein eindeutiges Ziel. Bei mehreren schnellen
      // Back-/Forward-Ereignissen darf ein verspäteter Zwischen-Commit nicht
      // den Balken für das zuletzt angeforderte Ziel beenden.
      const id = nextNavigationId + 1;
      nextNavigationId = id;
      update(() => {
        clearNavigations();
        clearHistoryNavigation();
        pendingHistoryNavigation = {
          id,
          target,
          timeout: setTimeout(() => {
            if (pendingHistoryNavigation?.id !== id) return;
            update(() => {
              pendingHistoryNavigation = null;
            });
          }, NAVIGATION_TIMEOUT_MS),
        };
      });
    },
    completeNavigation: (url) => {
      update(() => {
        if (pendingHistoryNavigation) {
          if (
            pendingHistoryNavigation.target === url ||
            // Ein Redirect ersetzt die History-URL. Nur wenn der Commit der
            // aktuellen Browser-URL entspricht, ist er kein verspäteter
            // Zwischen-Commit eines schnellen Back-/Forward-Wechsels.
            url === currentUrl()
          ) {
            clearHistoryNavigation();
          }
          return;
        }

        const matchingNavigation = pendingNavigations.findIndex(
          (pending) => pending.target === url,
        );
        if (matchingNavigation !== -1) {
          // Der App-Router bestätigt seine Warteschlange der Reihe nach. Ein
          // späteres, gleiches Ziel bleibt nach diesem Commit deshalb weiter
          // ausstehend.
          const completed = pendingNavigations.splice(
            0,
            matchingNavigation + 1,
          );
          for (const navigation of completed) {
            clearTimeout(navigation.timeout);
          }
        } else if (pendingNavigations.length === 1) {
          // Ein Redirect ändert die Ziel-URL. Ohne weitere ausstehende
          // Navigation gehört der Commit zu diesem Wechsel.
          const [completed] = pendingNavigations.splice(0, 1);
          if (completed) clearTimeout(completed.timeout);
        }
      });
    },
    cancelNavigation: (id) => {
      update(() => {
        if (pendingLinkNavigation?.id === id) {
          pendingLinkNavigation = null;
        }
        const index = pendingNavigations.findIndex(
          (pending) => pending.id === id,
        );
        if (index === -1) return;
        const [cancelled] = pendingNavigations.splice(index, 1);
        if (cancelled) clearTimeout(cancelled.timeout);
      });
    },
  };
}

export const NavigationProgressContext =
  createContext<NavigationProgressStore | null>(null);
const NOT_PENDING = () => false;
const NO_SUBSCRIPTION = () => () => undefined;

export function useNavigationProgressPending() {
  const store = useContext(NavigationProgressContext);
  return useSyncExternalStore(
    store?.subscribe ?? NO_SUBSCRIPTION,
    store?.isPending ?? NOT_PENDING,
    NOT_PENDING,
  );
}

export function useNavigationFallbackSuppressed() {
  const store = useContext(NavigationProgressContext);
  return useSyncExternalStore(
    store?.subscribe ?? NO_SUBSCRIPTION,
    store?.isFallbackSuppressed ?? NOT_PENDING,
    NOT_PENDING,
  );
}

/**
 * Umschließt die Hülle eines Portals. Außerhalb davon melden weder
 * `NavigationLink`s noch programmgesteuerte Wechsel etwas und der Balken
 * erscheint nie — Tests und Stories brauchen den Anbieter deshalb nicht.
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
  useEffect(() => {
    // Next 16 löst native `next/link`-Wechsel direkt über seine interne
    // Router-Aktion aus. Das Dokumentelement liegt in der Capture-Phase nach
    // `document`: Navigation Guards können den Klick dort zuerst abbrechen;
    // erfolgreiche Links werden noch vor Nexts Router-Dispatch gemeldet.
    const handleLinkClick = (event: MouseEvent) => {
      const target = linkNavigationTarget(event);
      if (target !== null && target !== currentUrl()) {
        store.startLinkNavigation(target);
      }
    };
    const root = document.documentElement;
    root.addEventListener("click", handleLinkClick, true);
    return () => root.removeEventListener("click", handleLinkClick, true);
  }, [store]);
  // Alle Nachkommen erhalten einen gleichartigen Router. So deckt die
  // Fortschrittsanzeige bestehende router.push/replace-Aufrufe ab, ohne jede
  // Schaltfläche auf einen eigenen Navigationshelfer umzustellen. Back und
  // Forward laufen unverändert durch und werden über `popstate` gemeldet.
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

  const id = store.startNavigation(target);
  try {
    navigate();
  } catch (error) {
    store.cancelNavigation(id);
    throw error;
  }
}

function navigationTarget(href: string): string | null {
  try {
    const target = new URL(href, window.location.href);
    if (target.origin !== window.location.origin) return null;
    return normalizedUrl(target.pathname, target.search);
  } catch {
    return null;
  }
}

function linkNavigationTarget(event: MouseEvent): string | null {
  if (
    event.defaultPrevented ||
    event.button !== 0 ||
    event.metaKey ||
    event.ctrlKey ||
    event.shiftKey ||
    event.altKey ||
    !(event.target instanceof Element)
  ) {
    return null;
  }

  const link = event.target.closest<HTMLAnchorElement>("a[href]");
  if (
    link === null ||
    (link.target !== "" && link.target !== "_self") ||
    link.hasAttribute("download")
  ) {
    return null;
  }
  return navigationTarget(link.href);
}

function currentUrl() {
  return normalizedUrl(window.location.pathname, window.location.search);
}

function normalizedUrl(pathname: string, search: string) {
  const normalizedSearch = new URLSearchParams(search).toString();
  return normalizedSearch === "" ? pathname : `${pathname}?${normalizedSearch}`;
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
    const url = normalizedUrl(pathname, search);
    store.completeNavigation(url);
    lastRouteUrl.current = url;
  }, [pathname, search, store]);

  useEffect(() => {
    const handleHistoryNavigation = () => {
      const url = currentUrl();
      if (lastRouteUrl.current !== null && url !== lastRouteUrl.current) {
        store.startHistory(url);
      }
    };
    window.addEventListener("popstate", handleHistoryNavigation);
    return () =>
      window.removeEventListener("popstate", handleHistoryNavigation);
  }, [store]);

  return null;
}

/**
 * Der Balken selbst. Er liegt fest am oberen Rand, verschiebt nichts und
 * erscheint erst nach 150 ms — kurze Wechsel bleiben dadurch unsichtbar
 * ruhig, statt kurz aufzublitzen.
 */
export function NavigationProgressBar() {
  const pending = useNavigationProgressPending();

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
