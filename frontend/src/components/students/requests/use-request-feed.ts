"use client";

/**
 * Das Nachladen der Anfragenliste (#2432, #2267). Zwei Quellen (der
 * Aggregator und die Anmeldungsänderungen) werden nach Zeitpunkt verschränkt;
 * die Abmeldungen laufen als eigener, seitenbasierter Abruf daneben.
 *
 * Seit #2267 zieht die offene Liste NICHT mehr blind alle Seiten nach: nach
 * der ersten Seite folgt höchstens eine weitere, und auch die nur, solange
 * alles Geladene heute dringend ist. Alles Weitere holt „Weitere Einträge
 * laden". Eine Schule mit hunderten offenen Anfragen hat sonst beim Öffnen
 * der Seite dutzende Abrufe ausgelöst.
 *
 * Seit #4988 liegen beide Listen im SWR-Cache, je Konto, Ansicht und Filter.
 * Wer die Seite erneut öffnet oder zurückkommt, sieht sofort den letzten
 * Stand; der Abgleich mit dem Server läuft im Hintergrund. Im Cache steht der
 * ganze Listenzustand mit den Cursors der Quellen, damit „Weitere Einträge
 * laden" auch nach dem Zurückkommen dort weitermacht, wo die Liste steht.
 * Entscheidungen und nachgeladene Seiten schreiben in denselben Eintrag.
 */

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { useSession } from "next-auth/react";

import { useApiLoadError } from "~/contexts/ToastContext";

import {
  fetchCareWithdrawals,
  type CareWithdrawalCompletion,
} from "~/lib/care-exit-api";
import {
  listAggregatedOpenRequests,
  listAggregatedRequestHistory,
  listEnrollmentChangeRequests,
  type AggregatedRequestParams,
  type AggregatedRequestType,
  type RequestReviewAccess,
} from "~/lib/change-request-list-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  cloneFeedState,
  createFeedState,
  takeMergedPage,
  type FeedPage,
  type FeedSource,
  type FeedState,
} from "~/lib/request-feed";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import type { AnyItem } from "./case-model";
import type { AggregatedRequestFilters } from "./filters";

const logger = createLogger({ component: "AggregatedRequestList" });

/** Objekt der Ladefehler im Katalogsatz (#2513). */
const LIST_OBJECT = "die Liste der Anfragen";
const WITHDRAWAL_OBJECT = "die Liste der Abmeldungen";

/**
 * Meldet einen Ladefehler vor Ort, mit Wiederholen (#2513). `null` leert ihn.
 */
export type ReportLoadError = (
  error: unknown,
  object: string,
  retry: () => void,
) => void;

/** Wie viele Zeilen eine Seite zeigt. */
const PAGE_SIZE = 25;
const WITHDRAWAL_PAGE_SIZE = 25;

/**
 * Wie viele Seiten die offene Liste ohne Zutun nachlädt. Eine erste Seite und
 * höchstens eine zweite — mehr wäre eine Sammelabfrage der ganzen Schule.
 */
const MAX_AUTO_PAGES = 2;

/**
 * Gemeinsame SWR-Optionen beider Listen. Die Liste zeigt beim Wechsel von
 * Ansicht oder Filter ihr Skelett statt der Zeilen des alten Filters, und
 * ein Fehlschlag wird nicht still wiederholt: der Fehler steht mit
 * „Wiederholen“ vor Ort. Fokus und Sichtbarkeit gleichen die offene Liste ab.
 */
function feedSwrOptions(view: "open" | "history") {
  return {
    keepPreviousData: false,
    shouldRetryOnError: false,
    revalidateOnFocus: view === "open",
    // Jedes erneute Öffnen gleicht ab, auch kurz nach dem letzten Abruf.
    dedupingInterval: 0,
  } as const;
}

/** Eine Seite einer Anfragequelle; der Aggregator meldet dazu die Freigabe. */
type RequestFeedPage = FeedPage<AnyItem> & {
  readonly review_access?: RequestReviewAccess;
};

interface RequestFeedSource extends FeedSource<AnyItem> {
  readonly fetchPage: (cursor?: string) => Promise<RequestFeedPage>;
}

/**
 * Der Listenzustand im Cache. `feed` hält die Cursors und Puffer der Quellen
 * nach den geladenen Seiten; er wird nie verändert, nur ersetzt.
 */
interface RequestFeedSnapshot extends PagedSnapshot<AnyItem> {
  readonly pages: number;
  readonly feed: FeedState<AnyItem>;
  readonly reviewAccess?: RequestReviewAccess;
}

/** Der Cache-Schlüssel: Konto, Ansicht und alles, was die Quellen abfragen. */
function requestFeedKey(
  name: string,
  accountId: string | undefined,
  view: "open" | "history",
  parts: readonly unknown[],
): string | null {
  if (!accountId) return null;
  return `${name}:${accountId}:${view}:${JSON.stringify(parts)}`;
}

export function useRequestSources(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
): RequestFeedSource[] {
  return useMemo<RequestFeedSource[]>(() => {
    const params: AggregatedRequestParams = {
      search: filters.search,
      studentId: filters.studentId,
      ...(view === "history"
        ? { statuses: filters.statuses, from: filters.from, to: filters.to }
        : {}),
    };
    const wantsType = (type: AggregatedRequestType) =>
      filters.types.length === 0 || filters.types.includes(type);
    const aggregatedTypes = filters.types.filter(
      (type) => type !== "enrollment" && type !== "care_withdrawal",
    );
    const sources: RequestFeedSource[] = [];
    if (
      filters.includeAggregated !== false &&
      (filters.types.length === 0 || aggregatedTypes.length > 0)
    ) {
      const aggregatedParams = { ...params, types: aggregatedTypes };
      sources.push({
        key: "aggregated",
        fetchPage: (cursor) =>
          view === "history"
            ? listAggregatedRequestHistory({ ...aggregatedParams, cursor })
            : listAggregatedOpenRequests({ ...aggregatedParams, cursor }),
      });
    }
    if (filters.includeEnrollment && wantsType("enrollment")) {
      sources.push({
        key: "enrollment",
        fetchPage: (cursor) =>
          listEnrollmentChangeRequests(view, { ...params, cursor }),
      });
    }
    return sources;
  }, [filters, view]);
}

/** Was die Quellen abfragen; bestimmt den Cache-Schlüssel der Liste. */
function requestFeedParts(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
): unknown[] {
  return [
    filters.search,
    filters.studentId ?? null,
    filters.includeAggregated !== false,
    filters.includeEnrollment === true,
    filters.types,
    view === "history" ? filters.statuses : [],
    view === "history" ? (filters.from ?? null) : null,
    view === "history" ? (filters.to ?? null) : null,
  ];
}

/** Erste Seite über alle Quellen; die Freigabe meldet der Aggregator mit. */
async function loadFirstRequestPage(
  sources: readonly RequestFeedSource[],
): Promise<RequestFeedSnapshot> {
  let reviewAccess: RequestReviewAccess | undefined;
  const reporting = sources.map((source) => ({
    key: source.key,
    fetchPage: async (cursor?: string) => {
      const page = await source.fetchPage(cursor);
      reviewAccess ??= page.review_access;
      return page;
    },
  }));
  const feed = createFeedState<AnyItem>(reporting);
  const page = await takeMergedPage(reporting, feed, PAGE_SIZE);
  return {
    snapshot: Symbol(),
    items: page.items,
    hasMore: page.hasMore,
    pages: 1,
    feed,
    reviewAccess,
  };
}

/**
 * Soll noch eine Seite ohne Zutun folgen? Nur solange die Obergrenze nicht
 * erreicht ist UND alles Geladene heute dringend ist — dann fehlt sonst
 * womöglich ein Kind, das heute betroffen ist.
 */
function shouldAutoLoad(pages: number, items: readonly AnyItem[]): boolean {
  if (pages >= MAX_AUTO_PAGES) return false;
  return (
    items.length > 0 &&
    items.every((item) => (item as { urgent_today?: boolean }).urgent_today)
  );
}

/**
 * Was eine Liste im Cache hält. `snapshot` kennzeichnet jede geladene erste
 * Seite einzeln: auch eine inhaltsgleiche beginnt einen neuen Stand, an den
 * keine Seite eines älteren mehr angehängt wird.
 */
interface PagedSnapshot<T> {
  readonly snapshot: symbol;
  readonly items: T[];
  readonly hasMore: boolean;
}

/**
 * Erste Seite, Nachladen und lokale Änderungen einer Liste im SWR-Cache.
 *
 * Koordiniert wird über den Cache-Eintrag, nicht über diese Instanz:
 * `isValidating` gilt je Schlüssel für alle Instanzen, auch für einen Abgleich,
 * den eine inzwischen verlassene Seite gestartet hat. SWR verwirft eine
 * laufende Antwort, sobald derselbe Schlüssel mutiert wird. Deshalb lädt die
 * Liste während eines Abgleichs nichts nach und hängt keine Seite an, und
 * eine Entscheidung während eines Abgleichs gleicht danach erneut ab.
 */
function usePagedFeed<T, S extends PagedSnapshot<T>>({
  key,
  family,
  view,
  object,
  logEvent,
  loadFirst,
  loadNext,
  autoLoad,
  updateSnapshot,
  reportError,
  clearError,
}: {
  key: string | null;
  /** Gemeinsamer Teil aller Schlüssel dieser Liste für das Konto. */
  family: string;
  view: "open" | "history";
  /** Objekt der Ladefehler im Katalogsatz. */
  object: string;
  logEvent: string;
  loadFirst: () => Promise<S>;
  /** Holt die nächste Seite zu `base` und liefert, wie sie angehängt wird. */
  loadNext: (base: S) => Promise<(current: S) => S>;
  /** Soll nach `snapshot` noch eine Seite ohne Zutun folgen? */
  autoLoad: (snapshot: S) => boolean;
  /** Wendet lokale Änderungen auch auf gepufferte Zeilen an. */
  updateSnapshot?: (current: S, update: (items: T[]) => T[]) => S;
  reportError: ReportLoadError;
  clearError: () => void;
}) {
  const {
    data,
    error: swrError,
    isValidating,
    mutate,
  } = useSWRAuth<S>(key, loadFirst, feedSwrOptions(view));
  const latest = useRef({ key, data, isValidating });
  useLayoutEffect(() => {
    latest.current = { key, data, isValidating };
    return () => {
      // Nach dem Unmount darf diese Instanz keinen neuen Abgleich verwerfen.
      latest.current = { key: null, data: undefined, isValidating: false };
    };
  });
  const updateFamily = useTenantMutateMatching([family]);
  const [loadingMore, setLoadingMore] = useState(false);
  const [autoLoadFailed, setAutoLoadFailed] = useState(false);
  // Laden und Abgleich im Hintergrund melden ihren Fehler über denselben
  // Fehlerpfad (#2513, #4988); die geladene Liste bleibt dabei stehen.
  const reload = useCallback(async () => {
    setAutoLoadFailed(false);
    await mutate();
    clearError();
  }, [clearError, mutate]);
  const error = useSwrLoadError(swrError, object, reload);
  useEffect(() => {
    setAutoLoadFailed(false);
    clearError();
  }, [clearError, key]);

  /** Ändert die geladenen Zeilen, z. B. nach einer Entscheidung. */
  const setItems = useCallback(
    (update: (current: T[]) => T[]) => {
      const apply = (current: S | undefined) =>
        current && {
          ...(updateSnapshot
            ? updateSnapshot(current, update)
            : { ...current, items: update(current.items) }),
          // Auch lokale Änderungen ersetzen den Stand laufender Folgeseiten.
          snapshot: Symbol(),
        };
      void mutate(apply, { revalidate: latest.current.isValidating });
      // Dieselbe Änderung gilt für alle Ansichten und Filter der Liste: eine
      // entschiedene Anfrage käme sonst beim Filterwechsel zurück.
      if (key)
        void updateFamily({
          update: (current) => apply(current as S | undefined),
          except: [key],
        });
    },
    [key, mutate, updateFamily, updateSnapshot],
  );

  const loadMoreRef = useRef<() => Promise<void>>(async () => undefined);
  const loadMore = useCallback(async () => {
    if (!data?.hasMore || loadingMore || isValidating) return;
    const base = data;
    // Die Seite gehört nur zu dem Stand, von dem aus sie geholt wurde.
    const isCurrent = () =>
      latest.current.key === key &&
      latest.current.data?.snapshot === base.snapshot &&
      !latest.current.isValidating;
    setLoadingMore(true);
    clearError();
    try {
      const append = await loadNext(base);
      if (!isCurrent()) return;
      await mutate(
        (current) =>
          current?.snapshot === base.snapshot ? append(current) : current,
        { revalidate: false },
      );
      setAutoLoadFailed(false);
    } catch (err) {
      // Der Fehler eines ersetzten Stands betrifft die aktuelle Liste nicht.
      if (!isCurrent()) return;
      logger.warn(logEvent, {
        error: err instanceof Error ? err.message : String(err),
      });
      reportError(err, object, () => void loadMoreRef.current());
      setAutoLoadFailed(true);
    } finally {
      setLoadingMore(false);
    }
  }, [
    clearError,
    data,
    isValidating,
    key,
    loadNext,
    loadingMore,
    logEvent,
    mutate,
    object,
    reportError,
  ]);
  useEffect(() => {
    loadMoreRef.current = loadMore;
  });
  useEffect(() => {
    if (
      view === "open" &&
      data?.hasMore &&
      !loadingMore &&
      !isValidating &&
      !autoLoadFailed &&
      autoLoad(data)
    )
      void loadMore();
  }, [
    autoLoad,
    autoLoadFailed,
    data,
    isValidating,
    loadMore,
    loadingMore,
    view,
  ]);

  return {
    data,
    items: data?.items,
    hasMore: data?.hasMore ?? false,
    loading: data === undefined && swrError === undefined,
    // Während eines Abgleichs wartet „Weitere Einträge laden“ auf dessen Stand.
    loadingMore: loadingMore || (isValidating && data !== undefined),
    error,
    reload,
    setItems,
    loadMore,
  };
}

export function useMergedRequestFeed(
  sources: readonly RequestFeedSource[],
  view: "open" | "history",
  filters: AggregatedRequestFilters,
) {
  const { data: session } = useSession();
  const accountId = session?.user.id;
  const loadError = useApiLoadError();
  const { error: pageError, clear: clearError } = loadError;
  const showLoadError = loadError.show;
  const reportError = useCallback<ReportLoadError>(
    (err, object, retry) => void showLoadError(err, { object, retry }),
    [showLoadError],
  );
  const loadNext = useCallback(
    async (base: RequestFeedSnapshot) => {
      // Eine Kopie: der Cache-Eintrag bleibt, wie er war, bis die Seite da ist.
      const feed = cloneFeedState(base.feed);
      const page = await takeMergedPage(sources, feed, PAGE_SIZE);
      return (current: RequestFeedSnapshot): RequestFeedSnapshot => ({
        ...current,
        items: [...current.items, ...page.items],
        hasMore: page.hasMore,
        pages: current.pages + 1,
        feed,
      });
    },
    [sources],
  );
  const autoLoad = useCallback(
    (snapshot: RequestFeedSnapshot) =>
      shouldAutoLoad(snapshot.pages, snapshot.items),
    [],
  );
  const feed = usePagedFeed<AnyItem, RequestFeedSnapshot>({
    key: requestFeedKey(
      "request-feed",
      accountId,
      view,
      requestFeedParts(view, filters),
    ),
    family: `request-feed:${accountId}:`,
    view,
    object: LIST_OBJECT,
    logEvent: "aggregated_request_list_load_more_failed",
    updateSnapshot: (current, update) => {
      const feed = cloneFeedState(current.feed);
      for (const source of Object.values(feed)) {
        source.buffer = update(source.buffer);
      }
      return { ...current, items: update(current.items), feed };
    },
    loadFirst: () => loadFirstRequestPage(sources),
    loadNext,
    autoLoad,
    reportError,
    clearError,
  });
  const items = useMemo(() => feed.items ?? [], [feed.items]);
  return {
    items,
    setItems: feed.setItems,
    hasMore: feed.hasMore,
    loading: feed.loading,
    loadingMore: feed.loadingMore,
    reviewAccess: feed.data?.reviewAccess,
    error: feed.error ?? pageError,
    clearError,
    reportError,
    reload: feed.reload,
    loadMore: feed.loadMore,
  };
}

async function fetchWithdrawalPage(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
  page: number,
) {
  if (
    !filters.includeCareWithdrawals ||
    (filters.types.length > 0 && !filters.types.includes("care_withdrawal"))
  )
    return { items: [], total: 0, page, pageSize: WITHDRAWAL_PAGE_SIZE };
  return fetchCareWithdrawals({
    search: filters.search,
    studentId: filters.studentId,
    page,
    pageSize: WITHDRAWAL_PAGE_SIZE,
    ...(view === "history" ? { state: "resolved" as const } : {}),
  });
}

/** Der Abmeldungen-Zustand im Cache; `nextPage` ist die nächste Seite. */
interface WithdrawalFeedSnapshot extends PagedSnapshot<CareWithdrawalCompletion> {
  readonly nextPage: number;
}

/** Alle Seiten der Abmeldungen nachladen: ein Fall ist erst vollständig. */
const ALWAYS = () => true;

export function useWithdrawalFeed(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
  reportError: ReportLoadError,
  clearError: () => void,
) {
  const { data: session } = useSession();
  const accountId = session?.user.id;
  const included =
    filters.includeCareWithdrawals === true &&
    (filters.types.length === 0 || filters.types.includes("care_withdrawal"));
  const loadNext = useCallback(
    async (base: WithdrawalFeedSnapshot) => {
      const pageNumber = base.nextPage;
      const page = await fetchWithdrawalPage(view, filters, pageNumber);
      return (current: WithdrawalFeedSnapshot): WithdrawalFeedSnapshot => ({
        ...current,
        items: [...current.items, ...page.items],
        hasMore: pageNumber * WITHDRAWAL_PAGE_SIZE < page.total,
        nextPage: pageNumber + 1,
      });
    },
    [filters, view],
  );
  const feed = usePagedFeed<CareWithdrawalCompletion, WithdrawalFeedSnapshot>({
    key: included
      ? requestFeedKey("care-withdrawal-feed", accountId, view, [
          filters.search,
          filters.studentId ?? null,
        ])
      : null,
    family: `care-withdrawal-feed:${accountId}:`,
    view,
    object: WITHDRAWAL_OBJECT,
    logEvent: "care_withdrawal_list_load_more_failed",
    loadFirst: async () => {
      const page = await fetchWithdrawalPage(view, filters, 1);
      return {
        snapshot: Symbol(),
        items: page.items,
        hasMore: page.items.length < page.total,
        nextPage: 2,
      };
    },
    loadNext,
    autoLoad: ALWAYS,
    reportError,
    clearError,
  });
  const items = useMemo(
    () => (included ? (feed.items ?? []) : []),
    [feed.items, included],
  );
  return {
    items,
    setItems: feed.setItems,
    loading: included && feed.loading,
    loadingMore: feed.loadingMore,
    hasMore: included && feed.hasMore,
    error: feed.error,
    load: feed.reload,
    loadMore: feed.loadMore,
  };
}
