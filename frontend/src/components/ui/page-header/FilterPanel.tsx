"use client";

import { useEffect, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";
import {
  normalizeFilterValues,
  type FilterPanelProps,
  type FilterConfig,
  type FilterOption,
  type FilterSection,
} from "./types";
import { InfoSection } from "../detail-modal-components";

// Viewport Y just below the sticky app topbar — the line the panel docks to.
// Measured live so it tracks the topbar's responsive + scroll-shrink heights.
// Falls back to an 8px inset when there is no topbar (tests / non-app-shell).
function topbarFloor(): number {
  const topbar =
    typeof document !== "undefined"
      ? document.querySelector("[data-app-header]")
      : null;
  return topbar ? Math.round(topbar.getBoundingClientRect().bottom) + 8 : 8;
}

const MOBILE_BOTTOM_NAV_CLEARANCE = "calc(6rem + env(safe-area-inset-bottom))";

function panelMaxHeight(top: number, isDesktopPlacement: boolean): string {
  return isDesktopPlacement
    ? `min(70vh, calc(100dvh - ${top}px - 1rem))`
    : `min(80vh, calc(100dvh - ${top}px - ${MOBILE_BOTTOM_NAV_CLEARANCE}))`;
}

// The panel is always `position: fixed`, docked just below the trigger but
// never higher than the topbar floor (so it stays visible once the trigger
// scrolls under the topbar). The live effect below re-measures on scroll/resize
// to keep it glued to the trigger; this same function seeds the first paint.
function panelStyleFor(
  rect: { left: number; bottom: number; right: number },
  isDesktopPlacement: boolean,
  floor: number,
): CSSProperties {
  const top = Math.max(floor, Math.round(rect.bottom + 8));
  const style: CSSProperties = {
    position: "fixed",
    top: `${top}px`,
    maxHeight: panelMaxHeight(top, isDesktopPlacement),
  };
  if (isDesktopPlacement) {
    const viewportWidth =
      typeof window !== "undefined" ? window.innerWidth : rect.right;
    style.left = `${Math.round(rect.left)}px`;
    style.right = `${Math.max(16, Math.round(viewportWidth - rect.right))}px`;
  }
  return style;
}

// Select an option. Multi-select toggles the value in/out of the current
// selection; single-select replaces it. Shared by every option-button layout.
function selectOption(
  filter: FilterConfig,
  optionValue: string,
  selectedValues: string[],
) {
  if (!filter.multiSelect) {
    filter.onChange(optionValue);
    return;
  }
  filter.onChange(
    selectedValues.includes(optionValue)
      ? selectedValues.filter((v) => v !== optionValue)
      : [...selectedValues, optionValue],
  );
}

// Map the consumer's sections onto the actual filters. Any filter no section
// claims falls into a trailing "Weitere" group so nothing is silently dropped.
function groupFiltersIntoSections(
  filters: FilterConfig[],
  sections: readonly FilterSection[],
) {
  const remaining = new Set(filters);
  const grouped = sections
    .map((section) => {
      const sectionFilters = filters.filter((filter) =>
        section.filterIds.includes(filter.id),
      );
      sectionFilters.forEach((filter) => remaining.delete(filter));
      return {
        title: section.title,
        icon: section.icon,
        filters: sectionFilters,
      };
    })
    .filter((section) => section.filters.length > 0);

  if (remaining.size > 0) {
    grouped.push({
      title: "Weitere",
      icon: undefined,
      filters: Array.from(remaining),
    });
  }

  return grouped;
}

export function FilterPanel({
  isOpen,
  onClose,
  filters,
  onApply,
  onReset,
  applyLabel = "Anwenden",
  placement = "mobile",
  anchorRect,
  anchorRef,
  variant = "default",
  sections,
  testId,
}: Readonly<FilterPanelProps>) {
  const isDesktopPlacement = placement === "desktop";
  const wasOpenRef = useRef(false);
  const isOpening = isOpen && !wasOpenRef.current;
  // Resolved fixed position. Seeded from the open-time anchor snapshot so the
  // first paint is correct, then the effect below re-measures on scroll/resize.
  const [posStyle, setPosStyle] = useState<CSSProperties | undefined>();

  useEffect(() => {
    wasOpenRef.current = isOpen;
  }, [isOpen]);

  // Escape key closes the panel — only attached while open so we don't pay
  // the listener cost on every page that mounts a panel.
  useEffect(() => {
    if (!isOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [isOpen, onClose]);

  // Track the trigger: re-measure on scroll (rAF-throttled) and resize, plus
  // when the topbar animates its height (mobile h-14↔h-12) which shifts the
  // trigger in document flow after the scroll settles.
  useEffect(() => {
    const trigger = anchorRef?.current;
    if (!isOpen || !trigger) return;
    let rafId = 0;
    let lastKey = "";
    const measure = () => {
      const rect = trigger.getBoundingClientRect();
      const next = panelStyleFor(rect, isDesktopPlacement, topbarFloor());
      const key = JSON.stringify(next);
      if (key === lastKey) return;
      lastKey = key;
      setPosStyle(next);
    };

    measure();
    const onScroll = () => {
      if (rafId !== 0) return;
      rafId = window.requestAnimationFrame(() => {
        rafId = 0;
        measure();
      });
    };
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("resize", measure);

    const topbarEl =
      typeof document !== "undefined"
        ? document.querySelector("[data-app-header]")
        : null;
    const observer =
      topbarEl && typeof ResizeObserver !== "undefined"
        ? new ResizeObserver(measure)
        : null;
    if (topbarEl && observer) observer.observe(topbarEl);

    return () => {
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("resize", measure);
      if (rafId !== 0) window.cancelAnimationFrame(rafId);
      observer?.disconnect();
    };
  }, [isOpen, anchorRef, isDesktopPlacement]);

  if (!isOpen) {
    return null;
  }

  const isQuiet = variant === "quiet";
  // Selected-pill treatment. Quiet mode swaps the near-black fill for the
  // app's calm blue-accent pill (same tokens as the active-filter badge in
  // PageHeaderWithSearch) so the panel matches the detail-modal language.
  const selectedOptionClass = isQuiet
    ? "bg-blue-50 text-blue-700 ring-1 ring-blue-200 ring-inset"
    : "bg-gray-900 text-white";

  // One pill button, shared by the buttons/grid/dropdown-multi layouts. The
  // layouts differ only in their wrapper, an optional leading icon (grid) and
  // an optional trailing count (dropdown) — passed in as `extraClass`/`content`.
  const optionButton = (
    filter: FilterConfig,
    option: FilterOption,
    selectedValues: string[],
    extraClass: string,
    content: React.ReactNode,
  ) => (
    <button
      key={option.value}
      type="button"
      onClick={() => selectOption(filter, option.value, selectedValues)}
      className={`rounded-lg px-3 py-2 text-sm font-medium transition-all ${extraClass} ${
        selectedValues.includes(option.value)
          ? selectedOptionClass
          : "bg-gray-50 text-gray-600 hover:bg-gray-100"
      } `}
    >
      {content}
    </button>
  );

  const renderFilterOptions = (filter: FilterConfig) => {
    const selectedValues = normalizeFilterValues(filter.value);
    switch (filter.type) {
      case "buttons":
        return (
          <div className="flex flex-wrap gap-1.5">
            {filter.options.map((option) =>
              optionButton(filter, option, selectedValues, "", option.label),
            )}
          </div>
        );

      case "grid":
        return (
          <div className="grid grid-cols-2 gap-2">
            {filter.options.map((option) =>
              optionButton(
                filter,
                option,
                selectedValues,
                "flex items-center gap-2",
                <>
                  {option.icon && (
                    <svg
                      className="h-4 w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d={option.icon}
                      />
                    </svg>
                  )}
                  {option.label}
                </>,
              ),
            )}
          </div>
        );

      case "dropdown":
        if (filter.multiSelect) {
          return (
            <div className="flex flex-wrap gap-1.5">
              {filter.options.map((option) =>
                optionButton(
                  filter,
                  option,
                  selectedValues,
                  "",
                  <>
                    {option.label}
                    {option.count !== undefined && (
                      <span
                        className={`ml-2 text-xs ${
                          selectedValues.includes(option.value)
                            ? isQuiet
                              ? "text-blue-500"
                              : "text-gray-300"
                            : "text-gray-500"
                        }`}
                      >
                        ({option.count})
                      </span>
                    )}
                  </>,
                ),
              )}
            </div>
          );
        }
        return (
          <select
            id={`mobile-filter-${filter.id}`}
            data-testid={`filter-${filter.id}`}
            value={filter.value as string}
            onChange={(event) => filter.onChange(event.target.value)}
            className={
              isQuiet
                ? "moto-select moto-content-surface h-10 w-full rounded-lg border px-3 text-sm font-medium text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                : "h-10 w-full rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm font-medium text-gray-900"
            }
          >
            {filter.options.map((option) => (
              <option key={option.value} value={option.value}>
                {option.count !== undefined
                  ? `${option.label} (${option.count})`
                  : option.label}
              </option>
            ))}
          </select>
        );

      default:
        return null;
    }
  };

  const renderFilterField = (filter: FilterConfig) => (
    <div key={filter.id}>
      <label
        htmlFor={`mobile-filter-${filter.id}`}
        className="mb-1.5 block text-sm font-semibold text-gray-800"
      >
        {filter.label}
      </label>
      {renderFilterOptions(filter)}
    </div>
  );

  const panelStyle =
    anchorRect && (isOpening || posStyle === undefined)
      ? panelStyleFor(anchorRect, isDesktopPlacement, topbarFloor())
      : posStyle;
  // Grouped layout is driven purely by the consumer-supplied sections — the
  // panel itself knows nothing about which filters belong together. No
  // sections → flat list.
  const useSections = sections !== undefined && sections.length > 0;
  const filterSections = useSections
    ? groupFiltersIntoSections(filters, sections)
    : [];
  // Anchored: `position` (fixed) + top come from panelStyle; the class only
  // carries the horizontal insets (mobile). Non-anchored: legacy bottom-sheet
  // fallback positioned entirely by the class.
  const panelPositionClass = anchorRect
    ? isDesktopPlacement
      ? ""
      : "inset-x-3 md:inset-x-auto md:right-4 md:w-96"
    : "fixed inset-x-3 bottom-[calc(6rem+env(safe-area-inset-bottom))] max-h-[calc(100dvh-7rem-env(safe-area-inset-bottom))] md:inset-x-auto md:top-[6rem] md:right-4 md:bottom-auto md:w-96";

  const panel = (
    <>
      {/* Click-outside backdrop. Transparent so the page content stays
          visible behind it — Stripe / Linear pattern. The panel itself
          dominates focus via shadow + border. Stays at z-20 (below the
          mobile bottom nav at z-30) on purpose: a higher backdrop would
          swallow taps on the nav and close the panel instead of letting
          the user navigate. */}
      <button
        type="button"
        onClick={onClose}
        aria-label="Filter schließen"
        className="fixed inset-0 z-20 cursor-default bg-transparent"
      />

      {/* Panel. When an anchor is provided by the search row, the portaled
          panel opens just below the trigger instead of covering it. The
          non-anchored branch keeps the legacy bottom-sheet fallback for
          standalone tests or older consumers. The panel sits at z-50 so it
          paints above page-level floating actions (FABs at z-40, e.g. the
          Activities create FAB or the binary-mode SchoolCheckinFab) instead
          of letting them cover the lower filter fields. It does NOT cover the
          mobile bottom nav (z-30): the height calculation reserves that nav
          area (MOBILE_BOTTOM_NAV_CLEARANCE), so the panel never reaches it
          geometrically — z-index only resolves the FAB overlap. */}
      <div
        role="dialog"
        // No aria-modal in either placement: the backdrop is transparent and
        // the page stays interactive (no dark scrim), so this is a non-modal
        // dialog. Claiming aria-modal would wrongly tell screen readers the
        // rest of the page is inert.
        aria-label="Filter"
        data-testid={testId}
        style={panelStyle}
        className={`z-50 flex flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-xl ${panelPositionClass}`}
      >
        <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4">
          {useSections ? (
            // Sectioned layout reuses the detail-modal InfoSection so the filter
            // panel speaks the same calm visual language as the room/enrollment
            // detail views — no bespoke section chrome.
            <div className="space-y-4">
              {filterSections.map((section) => (
                <InfoSection
                  key={section.title}
                  title={section.title}
                  icon={section.icon}
                  accentColor="gray"
                >
                  <div className="grid gap-3 md:grid-cols-2">
                    {section.filters.map(renderFilterField)}
                  </div>
                </InfoSection>
              ))}
            </div>
          ) : (
            <div className="space-y-4">{filters.map(renderFilterField)}</div>
          )}
        </div>

        {(onApply ?? onReset) && (
          <div className="flex gap-2 border-t border-gray-100 bg-white/95 p-3 backdrop-blur-sm">
            {onReset && (
              <button
                type="button"
                onClick={onReset}
                className="flex-1 py-2 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
              >
                Zurücksetzen
              </button>
            )}
            {onApply && (
              <button
                type="button"
                onClick={() => {
                  onApply();
                  onClose();
                }}
                className="flex-1 rounded-lg bg-gray-900 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
              >
                {applyLabel}
              </button>
            )}
          </div>
        )}
      </div>
    </>
  );

  // Portal to <body> so the panel escapes the page's `relative z-10` content
  // stacking context (app-shell.tsx). At the body level the transparent
  // backdrop stays at z-20 (above page content, below the z-30 bottom nav so
  // the nav stays tappable), while the panel itself is z-50 so it paints above
  // page FABs (z-40) without geometrically covering the bottom nav.
  if (typeof document !== "undefined") {
    return createPortal(panel, document.body);
  }

  return panel;
}
