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

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

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
import { useSWRAuth } from "~/lib/swr";
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
interface RequestFeedSnapshot {
  readonly items: AnyItem[];
  readonly hasMore: boolean;
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

export function useMergedRequestFeed(
  sources: readonly RequestFeedSource[],
  view: "open" | "history",
  filters: AggregatedRequestFilters,
) {
  const { data: session } = useSession();
  const key = requestFeedKey(
    "request-feed",
    session?.user.id,
    view,
    requestFeedParts(view, filters),
  );
  const {
    data,
    error: swrError,
    mutate,
  } = useSWRAuth<RequestFeedSnapshot>(
    key,
    () => loadFirstRequestPage(sources),
    feedSwrOptions(view),
  );
  const [loadingMore, setLoadingMore] = useState(false);
  const [autoLoadFailed, setAutoLoadFailed] = useState(false);
  const loadError = useApiLoadError();
  const { error: pageError, clear: clearError } = loadError;
  const showLoadError = loadError.show;
  const reportError = useCallback<ReportLoadError>(
    (err, object, retry) => void showLoadError(err, { object, retry }),
    [showLoadError],
  );
  // Laden und Abgleich im Hintergrund melden ihren Fehler über denselben
  // Fehlerpfad (#2513, #4988); die geladene Liste bleibt dabei stehen.
  const reload = useCallback(async () => {
    setAutoLoadFailed(false);
    await mutate();
  }, [mutate]);
  const listError = useSwrLoadError(swrError, LIST_OBJECT, reload);
  useEffect(() => setAutoLoadFailed(false), [key]);

  const items = useMemo(() => data?.items ?? [], [data]);
  const hasMore = data?.hasMore ?? false;
  const loading = data === undefined && swrError === undefined;

  /** Ändert die geladenen Zeilen, z. B. nach einer Entscheidung. */
  const setItems = useCallback(
    (update: (current: AnyItem[]) => AnyItem[]) => {
      void mutate(
        (current) => current && { ...current, items: update(current.items) },
        { revalidate: false },
      );
    },
    [mutate],
  );

  const loadMoreRef = useRef<() => Promise<void>>(async () => undefined);
  const loadMore = useCallback(async () => {
    if (!data?.hasMore || loadingMore) return;
    const base = data.feed;
    // Eine Kopie: der Cache-Eintrag bleibt, wie er war, bis die Seite da ist.
    const feed = cloneFeedState(base);
    setLoadingMore(true);
    clearError();
    try {
      const page = await takeMergedPage(sources, feed, PAGE_SIZE);
      await mutate(
        // Hat ein Neuladen die Liste inzwischen ersetzt, gehört die Seite zu
        // einem Stand, den niemand mehr sieht.
        (current) =>
          current?.feed === base
            ? {
                ...current,
                items: [...current.items, ...page.items],
                hasMore: page.hasMore,
                pages: current.pages + 1,
                feed,
              }
            : current,
        { revalidate: false },
      );
      setAutoLoadFailed(false);
    } catch (err) {
      logger.warn("aggregated_request_list_load_more_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      reportError(err, LIST_OBJECT, () => void loadMoreRef.current());
      setAutoLoadFailed(true);
    } finally {
      setLoadingMore(false);
    }
  }, [clearError, data, loadingMore, mutate, reportError, sources]);
  useEffect(() => {
    loadMoreRef.current = loadMore;
  });
  useEffect(() => {
    if (
      view === "open" &&
      data?.hasMore &&
      !loadingMore &&
      !autoLoadFailed &&
      shouldAutoLoad(data.pages, data.items)
    )
      void loadMore();
  }, [autoLoadFailed, data, loadMore, loadingMore, view]);
  return {
    items,
    setItems,
    hasMore,
    loading,
    loadingMore,
    reviewAccess: data?.reviewAccess,
    error: listError ?? pageError,
    clearError,
    reportError,
    reload,
    loadMore,
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

/**
 * Der Abmeldungen-Zustand im Cache; `nextPage` ist die nächste Seite, `key`
 * der Filtersatz, zu dem die Zeilen gehören.
 */
interface WithdrawalFeedSnapshot {
  readonly key: string;
  readonly items: CareWithdrawalCompletion[];
  readonly hasMore: boolean;
  readonly nextPage: number;
}

export function useWithdrawalFeed(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
  reportError: ReportLoadError,
  clearError: () => void,
) {
  const { data: session } = useSession();
  const included =
    filters.includeCareWithdrawals === true &&
    (filters.types.length === 0 || filters.types.includes("care_withdrawal"));
  const key = included
    ? requestFeedKey("care-withdrawal-feed", session?.user.id, view, [
        filters.search,
        filters.studentId ?? null,
      ])
    : null;
  const {
    data,
    error: swrError,
    mutate,
  } = useSWRAuth<WithdrawalFeedSnapshot>(
    key,
    async () => {
      const page = await fetchWithdrawalPage(view, filters, 1);
      return {
        key: key ?? "",
        items: page.items,
        hasMore: page.items.length < page.total,
        nextPage: 2,
      };
    },
    feedSwrOptions(view),
  );
  const [loadingMore, setLoadingMore] = useState(false);
  const [autoLoadFailed, setAutoLoadFailed] = useState(false);
  const load = useCallback(async () => {
    setAutoLoadFailed(false);
    await mutate();
  }, [mutate]);
  const error = useSwrLoadError(swrError, WITHDRAWAL_OBJECT, load);
  useEffect(() => setAutoLoadFailed(false), [key]);

  const items = useMemo(
    () => (included ? (data?.items ?? []) : []),
    [data, included],
  );
  const hasMore = included && (data?.hasMore ?? false);
  const loading = included && data === undefined && swrError === undefined;

  const setItems = useCallback(
    (
      update: (
        current: CareWithdrawalCompletion[],
      ) => CareWithdrawalCompletion[],
    ) => {
      void mutate(
        (current) => current && { ...current, items: update(current.items) },
        { revalidate: false },
      );
    },
    [mutate],
  );

  const loadMoreRef = useRef<() => Promise<void>>(async () => undefined);
  const loadMore = useCallback(async () => {
    if (!data?.hasMore || loadingMore) return;
    const base = data;
    setLoadingMore(true);
    clearError();
    try {
      const pageNumber = base.nextPage;
      const page = await fetchWithdrawalPage(view, filters, pageNumber);
      await mutate(
        // Nur an den Stand anhängen, von dem aus nachgeladen wurde: nach einem
        // Filterwechsel oder Neuladen gehört die Seite zu keiner Liste mehr.
        (current) =>
          current?.key === base.key && current.nextPage === pageNumber
            ? {
                ...current,
                items: [...current.items, ...page.items],
                hasMore: pageNumber * WITHDRAWAL_PAGE_SIZE < page.total,
                nextPage: pageNumber + 1,
              }
            : current,
        { revalidate: false },
      );
      setAutoLoadFailed(false);
    } catch (err) {
      logger.warn("care_withdrawal_list_load_more_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setAutoLoadFailed(true);
      reportError(err, WITHDRAWAL_OBJECT, () => void loadMoreRef.current());
    } finally {
      setLoadingMore(false);
    }
  }, [clearError, data, filters, loadingMore, mutate, reportError, view]);
  useEffect(() => {
    loadMoreRef.current = loadMore;
  });
  useEffect(() => {
    if (view === "open" && hasMore && !loadingMore && !autoLoadFailed)
      void loadMore();
  }, [autoLoadFailed, hasMore, loadMore, loadingMore, view]);
  return {
    items,
    setItems,
    loading,
    loadingMore,
    hasMore,
    error,
    load,
    loadMore,
  };
}
