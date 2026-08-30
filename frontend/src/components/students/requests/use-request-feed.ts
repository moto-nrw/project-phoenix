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
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

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
import { createLogger } from "~/lib/logger";
import {
  createFeedState,
  takeMergedPage,
  type FeedSource,
  type FeedState,
} from "~/lib/request-feed";
import type { AnyItem } from "./case-model";
import type { AggregatedRequestFilters } from "./filters";

const logger = createLogger({ component: "AggregatedRequestList" });

/** Wie viele Zeilen eine Seite zeigt. */
const PAGE_SIZE = 25;
const WITHDRAWAL_PAGE_SIZE = 25;

/**
 * Wie viele Seiten die offene Liste ohne Zutun nachlädt. Eine erste Seite und
 * höchstens eine zweite — mehr wäre eine Sammelabfrage der ganzen Schule.
 */
const MAX_AUTO_PAGES = 2;

export function useRequestSources(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
  onReviewAccess: (access: RequestReviewAccess) => void,
) {
  // Als Ref, damit ein neuer Callback die Quellen nicht neu erzeugt und damit
  // die ganze Liste neu lädt.
  const reportAccess = useRef(onReviewAccess);
  useEffect(() => {
    reportAccess.current = onReviewAccess;
  }, [onReviewAccess]);
  return useMemo<FeedSource<AnyItem>[]>(() => {
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
    const sources: FeedSource<AnyItem>[] = [];
    if (
      filters.includeAggregated !== false &&
      (filters.types.length === 0 || aggregatedTypes.length > 0)
    ) {
      const aggregatedParams = { ...params, types: aggregatedTypes };
      sources.push({
        key: "aggregated",
        fetchPage: async (cursor) => {
          if (view === "history") {
            return listAggregatedRequestHistory({
              ...aggregatedParams,
              cursor,
            });
          }
          const page = await listAggregatedOpenRequests({
            ...aggregatedParams,
            cursor,
          });
          if (page.review_access) reportAccess.current(page.review_access);
          return page;
        },
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

function useFeedLifecycle(sources: readonly FeedSource<AnyItem>[]) {
  const feedRef = useRef(createFeedState<AnyItem>(sources));
  const generationRef = useRef(0);
  const loadingRef = useRef(false);
  const loadMoreRef = useRef(false);
  const pagesRef = useRef(0);
  const start = useCallback(() => {
    const feed: FeedState<AnyItem> = createFeedState<AnyItem>(sources);
    const generation = ++generationRef.current;
    feedRef.current = feed;
    loadingRef.current = true;
    loadMoreRef.current = false;
    pagesRef.current = 1;
    const page = takeMergedPage(sources, feed, PAGE_SIZE).finally(() => {
      if (generation === generationRef.current) loadingRef.current = false;
    });
    return {
      generation,
      page,
      isCurrent: () => generation === generationRef.current,
    };
  }, [sources]);
  return { feedRef, generationRef, loadingRef, loadMoreRef, pagesRef, start };
}

function useInitialFeed(
  start: ReturnType<typeof useFeedLifecycle>["start"],
  setItems: (items: AnyItem[]) => void,
  setHasMore: (value: boolean) => void,
  setLoading: (value: boolean) => void,
  setError: (value: string | null) => void,
) {
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    const { page, isCurrent } = start();
    void page
      .then((result) => {
        if (cancelled || !isCurrent()) return;
        setItems(result.items);
        setHasMore(result.hasMore);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if (cancelled || !isCurrent()) return;
        logger.warn("aggregated_request_list_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Anfragen konnten nicht geladen werden.");
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [setError, setHasMore, setItems, setLoading, start]);
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
  sources: readonly FeedSource<AnyItem>[],
  view: "open" | "history",
) {
  const [items, setItems] = useState<AnyItem[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [autoLoadFailed, setAutoLoadFailed] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const lifecycle = useFeedLifecycle(sources);
  useEffect(() => setAutoLoadFailed(false), [sources, view]);
  useInitialFeed(lifecycle.start, setItems, setHasMore, setLoading, setError);
  const reload = useCallback(async () => {
    setAutoLoadFailed(false);
    const { generation, page } = lifecycle.start();
    try {
      const result = await page;
      if (generation !== lifecycle.generationRef.current) return;
      setItems(result.items);
      setHasMore(result.hasMore);
      setLoading(false);
    } catch (err) {
      if (generation !== lifecycle.generationRef.current) return;
      logger.warn("aggregated_request_list_reload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoading(false);
    }
  }, [lifecycle]);
  const loadMore = useCallback(async () => {
    if (
      !hasMore ||
      lifecycle.loadingRef.current ||
      lifecycle.loadMoreRef.current
    )
      return;
    const generation = lifecycle.generationRef.current;
    lifecycle.loadMoreRef.current = true;
    setLoadingMore(true);
    setError(null);
    try {
      const page = await takeMergedPage(
        sources,
        lifecycle.feedRef.current,
        PAGE_SIZE,
      );
      if (generation !== lifecycle.generationRef.current) return;
      lifecycle.pagesRef.current += 1;
      setItems((current) => [...current, ...page.items]);
      setHasMore(page.hasMore);
      setAutoLoadFailed(false);
    } catch (err) {
      if (generation === lifecycle.generationRef.current) {
        logger.warn("aggregated_request_list_load_more_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Weitere Anfragen konnten nicht geladen werden.");
        setAutoLoadFailed(true);
      }
    } finally {
      if (generation === lifecycle.generationRef.current) {
        lifecycle.loadMoreRef.current = false;
        setLoadingMore(false);
      }
    }
  }, [hasMore, lifecycle, sources]);
  useEffect(() => {
    if (
      view === "open" &&
      hasMore &&
      !loading &&
      !loadingMore &&
      !autoLoadFailed &&
      shouldAutoLoad(lifecycle.pagesRef.current, items)
    )
      void loadMore();
  }, [
    autoLoadFailed,
    hasMore,
    items,
    lifecycle,
    loadMore,
    loading,
    loadingMore,
    view,
  ]);
  return {
    items,
    setItems,
    hasMore,
    loading,
    loadingMore,
    error,
    setError,
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

export function useWithdrawalFeed(
  view: "open" | "history",
  filters: AggregatedRequestFilters,
  reportError: (message: string | null) => void,
) {
  const [items, setItems] = useState<CareWithdrawalCompletion[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [autoLoadFailed, setAutoLoadFailed] = useState(false);
  const nextPageRef = useRef(2);
  const generationRef = useRef(0);
  const load = useCallback(async () => {
    const generation = ++generationRef.current;
    setAutoLoadFailed(false);
    setLoadingMore(false);
    try {
      const page = await fetchWithdrawalPage(view, filters, 1);
      if (generation !== generationRef.current) return;
      setItems(page.items);
      setHasMore(page.items.length < page.total);
      nextPageRef.current = 2;
    } catch (err) {
      if (generation !== generationRef.current) return;
      logger.warn("care_withdrawal_list_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      reportError("Abmeldungen konnten nicht geladen werden.");
    }
  }, [filters, reportError, view]);
  useEffect(() => {
    let cancelled = false;
    setLoading(filters.includeCareWithdrawals === true);
    void load().finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [filters.includeCareWithdrawals, load]);
  const loadMore = useCallback(async () => {
    if (!hasMore || loadingMore) return;
    const generation = generationRef.current;
    setLoadingMore(true);
    reportError(null);
    try {
      const pageNumber = nextPageRef.current;
      const page = await fetchWithdrawalPage(view, filters, pageNumber);
      if (generation !== generationRef.current) return;
      setItems((current) => [...current, ...page.items]);
      setHasMore(pageNumber * WITHDRAWAL_PAGE_SIZE < page.total);
      nextPageRef.current = pageNumber + 1;
      setAutoLoadFailed(false);
    } catch {
      if (generation === generationRef.current) {
        setAutoLoadFailed(true);
        reportError("Weitere Abmeldungen konnten nicht geladen werden.");
      }
    } finally {
      if (generation === generationRef.current) setLoadingMore(false);
    }
  }, [filters, hasMore, loadingMore, reportError, view]);
  useEffect(() => {
    if (
      view === "open" &&
      hasMore &&
      !loading &&
      !loadingMore &&
      !autoLoadFailed
    )
      void loadMore();
  }, [autoLoadFailed, hasMore, loadMore, loading, loadingMore, view]);
  return { items, setItems, loading, loadingMore, hasMore, load, loadMore };
}
