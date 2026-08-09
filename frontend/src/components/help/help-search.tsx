"use client";

import {
  useCallback,
  useDeferredValue,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import { CornerDownLeft, FileText } from "lucide-react";
import Fuse from "fuse.js";
import { SearchBar } from "~/components/ui/page-header/SearchBar";
import {
  fold,
  foldForMatch,
  indexedHelpSearchIndex,
  type HelpSearchRecord,
} from "./guide-search";

const MAX_RESULTS = 12;
const MIN_QUERY_LENGTH = 2;
/**
 * Hard cap on the dropdown height (px). Keeps it a compact, scrollable box of
 * ~5 results on EVERY screen — without this, a tall monitor makes the panel
 * grow to fit all results, so it fills half the viewport and never scrolls.
 */
const MAX_PANEL_HEIGHT = 440;

/** One ranked search hit plus the highlight ranges for its display fields. */
export interface HelpSearchHit {
  readonly record: HelpSearchRecord;
  /** Inclusive [start, end] ranges into `record.title` to emphasise. */
  readonly titleRanges: readonly (readonly [number, number])[];
  /** Inclusive [start, end] ranges into `record.summary` to emphasise. */
  readonly summaryRanges: readonly (readonly [number, number])[];
}

/**
 * Compute highlight ranges for `text` from the typed query tokens. We do NOT
 * use Fuse's fuzzy match indices — those scatter across non-adjacent characters
 * and visually "deform" the word. Instead we mark only clean, contiguous
 * occurrences of each token (diacritic-insensitive). Tokens that match a card
 * only fuzzily (e.g. "armbnd") simply aren't highlighted — better no mark than a
 * jagged one. {@link fold} is length-preserving for precomposed characters, so
 * folded indices map straight onto the original text.
 */
function highlightRanges(
  text: string,
  tokens: readonly string[],
): [number, number][] {
  const folded = fold(text);
  const ranges: [number, number][] = [];
  for (const token of tokens) {
    let from = folded.indexOf(token);
    while (from !== -1) {
      ranges.push([from, from + token.length - 1]);
      from = folded.indexOf(token, from + token.length);
    }
  }
  return ranges;
}

/**
 * Single Fuse instance over the precomputed folded index. The index is static
 * for the app lifetime, so we build it once at module load rather than per hook
 * instance (the landing page mounts both the inline box and the palette).
 */
const helpFuse = new Fuse(indexedHelpSearchIndex, {
  includeScore: true,
  ignoreLocation: true,
  threshold: 0.4,
  minMatchCharLength: MIN_QUERY_LENGTH,
  keys: [
    { name: "titleFold", weight: 0.5 },
    { name: "chapterFold", weight: 0.2 },
    { name: "summaryFold", weight: 0.2 },
    { name: "bodyFold", weight: 0.1 },
  ],
});

type HelpFuse = typeof helpFuse;

/**
 * Run the fuzzy search for one query string and return ranked hits.
 *
 * Pulled out of the hook so it is a pure function of (fuse, query): each query
 * word is searched independently and the per-card results are merged, so a
 * multi-word query like "Kind finden" still surfaces "Kindersuche" (matched by
 * "kind") even though "finden" never appears verbatim. Ranking then layers
 * explicit boosts on top of Fuse's score so an exact/prefix title match always
 * beats an incidental body match.
 */
function runSearch(fuse: HelpFuse, rawQuery: string): HelpSearchHit[] {
  const trimmed = rawQuery.trim();
  // Two tokenisations: aggressive (ß→ss) for matching, length-preserving for
  // highlight offset maths.
  const matchTokens = foldForMatch(trimmed)
    .split(/\s+/)
    .filter((t) => t.length >= MIN_QUERY_LENGTH);
  const highlightTokens = fold(trimmed)
    .split(/\s+/)
    .filter((t) => t.length >= MIN_QUERY_LENGTH);
  if (matchTokens.length === 0) return [];

  const fullFold = foldForMatch(trimmed);

  const agg = new Map<
    string,
    {
      indexed: (typeof indexedHelpSearchIndex)[number];
      score: number;
      tokenHits: number;
    }
  >();

  for (const token of matchTokens) {
    for (const result of fuse.search(token)) {
      const score = result.score ?? 1;
      const existing = agg.get(result.item.record.id);
      if (existing) {
        existing.tokenHits += 1;
        existing.score = Math.min(existing.score, score);
      } else {
        agg.set(result.item.record.id, {
          indexed: result.item,
          score,
          tokenHits: 1,
        });
      }
    }
  }

  return [...agg.values()]
    .map((entry) => {
      const titleFold = entry.indexed.titleFold;
      let boost = 0;
      if (titleFold === fullFold)
        boost += 1000; // exact title
      else if (titleFold.startsWith(fullFold))
        boost += 400; // title prefix
      else if (titleFold.includes(fullFold)) boost += 200; // phrase in title
      if (entry.tokenHits === matchTokens.length) boost += 100; // all words matched
      // Word-aligned title hits beat substring-in-word hits: "raum" as a
      // prefix of the title word "raume" outranks "raum" buried inside
      // "kalenderzeitraume".
      const titleWords = titleFold.split(/\s+/);
      for (const token of matchTokens) {
        if (titleWords.some((word) => word.startsWith(token))) boost += 50;
      }
      return { ...entry, boost };
    })
    .sort(
      (a, b) =>
        b.boost - a.boost ||
        b.tokenHits - a.tokenHits ||
        a.score - b.score ||
        a.indexed.record.title.localeCompare(b.indexed.record.title),
    )
    .slice(0, MAX_RESULTS)
    .map((entry) => ({
      record: entry.indexed.record,
      titleRanges: highlightRanges(entry.indexed.record.title, highlightTokens),
      summaryRanges: highlightRanges(
        entry.indexed.record.summary,
        highlightTokens,
      ),
    }));
}

/** useLayoutEffect on the client, useEffect on the server (avoids SSR warning). */
const useIsomorphicLayoutEffect =
  typeof window !== "undefined" ? useLayoutEffect : useEffect;

/** Fixed-viewport placement for the dropdown, anchored under the search input. */
interface PanelRect {
  /** Viewport coordinates (the panel is `position: fixed`). */
  readonly top: number;
  readonly left: number;
  readonly width: number;
  /** Cap so the panel never runs past the viewport's bottom edge. */
  readonly maxHeight: number;
}

/**
 * Measure where the floating result panel should sit, anchored to the search
 * input (`anchorRef`). The panel is rendered in a portal with `position: fixed`,
 * so it is positioned in VIEWPORT coordinates and is completely independent of
 * the surrounding page — no ancestor `overflow`, page height, or transformed
 * containing block can affect it. That is what makes the dropdown behave 1:1
 * identically on every help page. Recomputed on open, resize, and scroll.
 */
function usePanelRect(
  anchorRef: React.RefObject<HTMLElement | null>,
  open: boolean,
): PanelRect | undefined {
  const [rect, setRect] = useState<PanelRect>();

  useIsomorphicLayoutEffect(() => {
    if (!open) {
      setRect(undefined);
      return;
    }
    const measure = () => {
      const el = anchorRef.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      const top = r.bottom + 12; // 12px gap below the input
      // Compact, fixed cap so it's a scrollable box on tall screens too, but
      // shrink further on short viewports (leave 16px above the bottom edge).
      const available = window.innerHeight - top - 16;
      setRect({
        top,
        left: r.left,
        width: r.width,
        maxHeight: Math.max(160, Math.min(MAX_PANEL_HEIGHT, available)),
      });
    };
    measure();
    window.addEventListener("resize", measure);
    window.addEventListener("scroll", measure, { passive: true });
    return () => {
      window.removeEventListener("resize", measure);
      window.removeEventListener("scroll", measure);
    };
  }, [anchorRef, open]);

  return rect;
}

/**
 * Fuzzy help search over the prebuilt index. The query is deferred with
 * {@link useDeferredValue} so typing always stays responsive: React renders the
 * new input value immediately and recomputes the (heavier) result list at lower
 * priority. Searches the shared module-level {@link helpFuse} instance.
 */
export function useHelpSearch(): {
  query: string;
  setQuery: (value: string) => void;
  deferredQuery: string;
  hits: HelpSearchHit[];
} {
  const [query, setQuery] = useState("");
  const deferredQuery = useDeferredValue(query);

  const hits = useMemo(
    () => runSearch(helpFuse, deferredQuery),
    [deferredQuery],
  );

  return { query, setQuery, deferredQuery, hits };
}

/** Render a string with the given ranges wrapped in <mark>. */
function Highlighted({
  text,
  ranges,
}: {
  readonly text: string;
  readonly ranges: readonly (readonly [number, number])[];
}) {
  if (ranges.length === 0) return <>{text}</>;

  // Ranges may arrive unsorted / overlapping (merged from several query tokens).
  // Sort by start and coalesce touching or overlapping ranges before rendering.
  const merged: [number, number][] = [];
  for (const [s, e] of [...ranges].sort((a, b) => a[0] - b[0])) {
    const last = merged[merged.length - 1];
    if (last && s <= last[1] + 1) {
      last[1] = Math.max(last[1], e);
    } else {
      merged.push([s, e]);
    }
  }

  const nodes: ReactNode[] = [];
  let cursor = 0;
  for (const [start, end] of merged) {
    if (start > cursor) nodes.push(text.slice(cursor, start));
    nodes.push(
      <mark
        key={`${start}-${end}`}
        className="bg-moto-green/25 rounded px-0.5 text-gray-950"
      >
        {text.slice(start, end + 1)}
      </mark>,
    );
    cursor = end + 1;
  }
  if (cursor < text.length) nodes.push(text.slice(cursor));
  return <>{nodes}</>;
}

/**
 * Shared result list used by both the inline landing box and the palette.
 * Keyboard navigation is driven from the parent via `activeIndex`; each row is
 * a Link so mouse, Enter, and middle-click all behave like normal navigation.
 */
function HelpSearchResults({
  hits,
  activeIndex,
  onActivate,
  onNavigate,
  linkRefs,
  listId,
  optionId,
}: {
  readonly hits: readonly HelpSearchHit[];
  readonly activeIndex: number;
  readonly onActivate: (index: number) => void;
  readonly onNavigate: () => void;
  readonly linkRefs: React.MutableRefObject<(HTMLAnchorElement | null)[]>;
  readonly listId: string;
  readonly optionId: (index: number) => string;
}) {
  return (
    <ul id={listId} role="listbox" className="space-y-1">
      {hits.map((hit, index) => {
        const active = index === activeIndex;
        return (
          <li key={hit.record.id} role="presentation">
            <Link
              ref={(el) => {
                linkRefs.current[index] = el;
              }}
              id={optionId(index)}
              role="option"
              aria-selected={active}
              tabIndex={-1}
              href={hit.record.href}
              onMouseEnter={() => onActivate(index)}
              onClick={onNavigate}
              className={`flex items-start gap-3 rounded-xl border px-3 py-2.5 transition-colors focus-visible:outline-none ${
                active
                  ? "border-moto-green/40 bg-moto-green/8"
                  : "border-transparent hover:bg-gray-50"
              }`}
            >
              <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500">
                <FileText className="h-3.5 w-3.5" aria-hidden="true" />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-semibold text-gray-950">
                  <Highlighted
                    text={hit.record.title}
                    ranges={hit.titleRanges}
                  />
                </span>
                <span className="mt-0.5 block truncate text-xs text-gray-600">
                  <Highlighted
                    text={hit.record.summary}
                    ranges={hit.summaryRanges}
                  />
                </span>
                <span className="mt-1 block truncate text-[11px] font-medium text-gray-400">
                  {hit.record.guideLabel} › {hit.record.chapterTitle}
                </span>
              </span>
              {active ? (
                <CornerDownLeft
                  className="mt-1 h-3.5 w-3.5 shrink-0 text-gray-400"
                  aria-hidden="true"
                />
              ) : null}
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

/** Clamp + reset active index, and keep the active row scrolled into view. */
function useActiveIndex(count: number): {
  activeIndex: number;
  setActiveIndex: (index: number) => void;
  move: (delta: number) => void;
  jump: (index: number) => void;
  linkRefs: React.MutableRefObject<(HTMLAnchorElement | null)[]>;
} {
  const [activeIndex, setActiveIndex] = useState(0);
  const linkRefs = useRef<(HTMLAnchorElement | null)[]>([]);

  useEffect(() => {
    setActiveIndex(0);
  }, [count]);

  const jump = useCallback(
    (index: number) => {
      if (count === 0) return;
      const clamped = Math.max(0, Math.min(index, count - 1));
      setActiveIndex(clamped);
      linkRefs.current[clamped]?.scrollIntoView({ block: "nearest" });
    },
    [count],
  );

  const move = useCallback(
    (delta: number) => {
      if (count === 0) return;
      setActiveIndex((prev) => {
        const next = (prev + delta + count) % count;
        linkRefs.current[next]?.scrollIntoView({ block: "nearest" });
        return next;
      });
    },
    [count],
  );

  return { activeIndex, setActiveIndex, move, jump, linkRefs };
}

/**
 * Shared listbox keyboard handling for both the inline box and the palette:
 * ↑/↓ move, Home/End jump, Enter opens the active row, Esc dismisses. Factored
 * out so the two consumers can't drift apart.
 */
function handleListboxKeys(
  event: React.KeyboardEvent,
  opts: {
    count: number;
    move: (delta: number) => void;
    jump: (index: number) => void;
    activeIndex: number;
    linkRefs: React.MutableRefObject<(HTMLAnchorElement | null)[]>;
    onEscape: () => void;
  },
): void {
  switch (event.key) {
    case "ArrowDown":
      event.preventDefault();
      opts.move(1);
      break;
    case "ArrowUp":
      event.preventDefault();
      opts.move(-1);
      break;
    case "Home":
      event.preventDefault();
      opts.jump(0);
      break;
    case "End":
      event.preventDefault();
      opts.jump(opts.count - 1);
      break;
    case "Enter":
      if (opts.count === 0) break;
      event.preventDefault();
      opts.linkRefs.current[opts.activeIndex]?.click();
      break;
    case "Escape":
      event.preventDefault();
      opts.onEscape();
      break;
    default:
      break;
  }
}

/**
 * The full-width inline help search box. Used both on the help landing page and
 * at the top of each guide page: a prominent field with a live result list
 * underneath. Uses the shared {@link SearchBar} so the styling matches the rest
 * of the app, wired up as an ARIA combobox for screen readers.
 */
export function HelpSearchInline() {
  const { query, setQuery, deferredQuery, hits } = useHelpSearch();
  const trimmed = query.trim();
  const canSearch = trimmed.length >= MIN_QUERY_LENGTH;
  const searchIsCurrent = deferredQuery.trim() === trimmed;
  const visibleHits = searchIsCurrent ? hits : [];
  const { activeIndex, setActiveIndex, move, jump, linkRefs } = useActiveIndex(
    visibleHits.length,
  );
  const [dismissed, setDismissed] = useState(false);
  const [mounted, setMounted] = useState(false);
  const containerRef = useRef<HTMLElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

  const baseId = useId();
  const listId = `${baseId}-results`;
  const optionId = (i: number) => `${baseId}-option-${i}`;

  const open = !dismissed && canSearch && visibleHits.length > 0;
  const showEmpty =
    !dismissed && canSearch && searchIsCurrent && visibleHits.length === 0;
  const panelVisible = open || showEmpty;

  const panelRect = usePanelRect(containerRef, panelVisible);

  // Portals require a DOM target, so only render the dropdown after mount.
  useEffect(() => setMounted(true), []);

  // While the panel is open, lock page scrolling so the wheel/trackpad scrolls
  // the result list rather than the page. Compensate for the reserved scrollbar
  // width so locking doesn't shift the layout sideways.
  useEffect(() => {
    if (!panelVisible) return;
    const root = document.documentElement;
    const prevOverflow = root.style.overflow;
    const prevPadding = root.style.paddingRight;
    const scrollbarWidth = window.innerWidth - root.clientWidth;
    root.style.overflow = "hidden";
    if (scrollbarWidth > 0) root.style.paddingRight = `${scrollbarWidth}px`;
    return () => {
      root.style.overflow = prevOverflow;
      root.style.paddingRight = prevPadding;
    };
  }, [panelVisible]);

  // Dismiss on an outside press (keeps the typed query) or Escape. The panel
  // lives in a portal OUTSIDE `containerRef`, so we must treat presses inside
  // either the search box or the panel as "inside" — otherwise clicking a
  // result would dismiss the list before the navigation click lands.
  useEffect(() => {
    if (!panelVisible) return;
    const onPointer = (event: MouseEvent | TouchEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (containerRef.current?.contains(target)) return;
      if (panelRef.current?.contains(target)) return;
      setDismissed(true);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setDismissed(true);
    };
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("touchstart", onPointer, { passive: true });
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("touchstart", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [panelVisible]);

  const handleChange = useCallback(
    (value: string) => {
      setDismissed(false);
      setQuery(value);
    },
    [setQuery],
  );

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent) => {
      if (!open) return;
      handleListboxKeys(event, {
        count: visibleHits.length,
        move,
        jump,
        activeIndex,
        linkRefs,
        onEscape: () => setDismissed(true),
      });
    },
    [activeIndex, jump, linkRefs, move, open, visibleHits.length],
  );

  // Shared fixed-position style for the floating panel (viewport coordinates).
  const panelStyle: React.CSSProperties | undefined = panelRect
    ? {
        position: "fixed",
        top: panelRect.top,
        left: panelRect.left,
        width: panelRect.width,
      }
    : undefined;

  return (
    <search ref={containerRef}>
      <SearchBar
        value={query}
        onChange={handleChange}
        placeholder="Anleitung durchsuchen…"
        size="lg"
        inputProps={{
          role: "combobox",
          "aria-expanded": open,
          "aria-controls": open ? listId : undefined,
          "aria-activedescendant": open ? optionId(activeIndex) : undefined,
          "aria-autocomplete": "list",
          "aria-label": "Anleitung durchsuchen",
          onKeyDown: handleKeyDown,
          onFocus: () => setDismissed(false),
        }}
      />
      {mounted && open && panelRect
        ? // Floating dropdown rendered in a body portal with `position: fixed`,
          // so it is anchored to the viewport and entirely independent of the
          // surrounding page — identical behaviour on every help page. The
          // outer box clips the rounded corners; the inner element scrolls so
          // the scrollbar stays inside the rounded box.
          createPortal(
            <div
              ref={panelRef}
              style={panelStyle}
              className="z-[60] overflow-hidden rounded-2xl border border-gray-200 bg-white/95 shadow-lg backdrop-blur-md"
            >
              <div
                style={{ maxHeight: panelRect.maxHeight }}
                className="overflow-y-auto overscroll-contain p-2"
              >
                <HelpSearchResults
                  hits={visibleHits}
                  activeIndex={activeIndex}
                  onActivate={setActiveIndex}
                  onNavigate={() => setQuery("")}
                  linkRefs={linkRefs}
                  listId={listId}
                  optionId={optionId}
                />
              </div>
            </div>,
            document.body,
          )
        : null}
      {mounted && showEmpty && panelRect
        ? createPortal(
            <p
              style={panelStyle}
              className="z-[60] rounded-2xl border border-gray-200 bg-white/95 px-4 py-6 text-center text-sm text-gray-500 shadow-lg backdrop-blur-md"
            >
              Keine Treffer für „{trimmed}“.
            </p>,
            document.body,
          )
        : null}
    </search>
  );
}

/**
 * Corrects anchor landing on guide pages. The step cards carry `loading="lazy"`
 * images, so on a fresh `#card-id` deep-link the images above the target have
 * zero height when the browser first scrolls — the target ends up off, "between
 * two cards". This re-scrolls to the hash on mount (after layout settles) and on
 * every hashchange, respecting each card's `scroll-margin-top`. It also
 * re-scrolls when lazy images above the target load after the fixed retry
 * window. Render once per guide page.
 */
export function HelpHashScroll() {
  useEffect(() => {
    const timers: number[] = [];
    let raf = 0;

    const clearScheduledScrolls = () => {
      if (raf) {
        cancelAnimationFrame(raf);
        raf = 0;
      }
      while (timers.length > 0) {
        const timer = timers.pop();
        if (timer !== undefined) clearTimeout(timer);
      }
    };

    const getHashTarget = () => {
      const id = decodeURIComponent(window.location.hash.replace("#", ""));
      if (!id) return null;
      return document.getElementById(id);
    };

    const scrollToHash = () => {
      getHashTarget()?.scrollIntoView();
    };

    const scrollAfterImageLoad = (event: Event) => {
      if (!(event.target instanceof HTMLImageElement)) return;

      const target = getHashTarget();
      if (!target) return;

      const position = event.target.compareDocumentPosition(target);
      if ((position & Node.DOCUMENT_POSITION_FOLLOWING) === 0) return;

      scrollToHash();
    };

    const scheduleHashScrolls = () => {
      clearScheduledScrolls();
      if (!window.location.hash) return;
      // Re-scroll as lazy images load and the layout above the target grows.
      scrollToHash();
      raf = requestAnimationFrame(scrollToHash);
      timers.push(window.setTimeout(scrollToHash, 150));
      timers.push(window.setTimeout(scrollToHash, 600));
    };

    scheduleHashScrolls();
    // Same-page result clicks only change the hash — catch those too. `load`
    // fires after this effect on a fresh deep-link (effect runs mid-hydration
    // while resources are still loading), so it's a real first-paint fallback.
    window.addEventListener("hashchange", scheduleHashScrolls);
    window.addEventListener("load", scrollToHash);
    document.addEventListener("load", scrollAfterImageLoad, true);
    return () => {
      clearScheduledScrolls();
      window.removeEventListener("hashchange", scheduleHashScrolls);
      window.removeEventListener("load", scrollToHash);
      document.removeEventListener("load", scrollAfterImageLoad, true);
    };
  }, []);

  return null;
}
