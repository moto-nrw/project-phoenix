"use client";

import { useId } from "react";
import { ChevronDown } from "lucide-react";
import type { SchemaCategory } from "~/lib/settings-api";
import { cn } from "~/lib/utils";
import { SettingsField } from "./settings-field";
import {
  categorySummary,
  changedCount,
  displayCategoryLabel,
  filterCategoryItems,
} from "./settings-filter";

interface SettingsCategoryProps {
  readonly category: SchemaCategory;
  readonly highlightKey?: string | null;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
  readonly onSchemaRefresh?: () => void;
  readonly onBookingAuthorityEnable?: () => Promise<void>;
  // audience identifies who is viewing the settings page. Controls the
  // "auch von {other side} änderbar" hint on shared settings. Defaults
  // to "admin" for the tenant settings page; the operator page passes
  // "operator" to flip the hint copy.
  readonly audience?: "admin" | "operator";
  // revealFn is forwarded to PasswordField. Defaults to the tenant reveal
  // endpoint when unset. The operator page passes a school-bound function
  // so password reveal hits the operator endpoint instead.
  readonly revealFn?: (key: string) => Promise<string | null>;
  /**
   * Renders the heading as a disclosure toggle (#2830). The open state is
   * controlled by the parent via `collapsed` + `onToggle`, so a tab can
   * expand the category that holds a deep-linked setting or expand every
   * category at once. Off by default: the operator page and the isolated
   * component keep the flat, always-open card.
   */
  readonly collapsible?: boolean;
  readonly collapsed?: boolean;
  readonly onToggle?: () => void;
  /**
   * Lower-cased search query. When set, only matching settings are listed
   * (all of them when the category name itself matches). The category
   * renders nothing when no setting matches.
   */
  readonly filterQuery?: string;
  /** Small label above the heading, e.g. the tab name in search results. */
  readonly kicker?: string;
}

export function SettingsCategory({
  category,
  highlightKey,
  onSave,
  onReset,
  onSchemaRefresh,
  onBookingAuthorityEnable,
  audience = "admin",
  revealFn,
  collapsible = false,
  collapsed = false,
  onToggle,
  filterQuery = "",
  kicker,
}: SettingsCategoryProps) {
  const panelId = useId();
  const visibleItems = filterCategoryItems(category, filterQuery);

  if (visibleItems.length === 0) {
    return null;
  }

  const label = displayCategoryLabel(category);
  const isCollapsed = collapsible && collapsed;
  const changed = changedCount(visibleItems);
  const changedBadge =
    changed > 0 ? (
      <span className="shrink-0 rounded bg-gray-100 px-1.5 py-0.5 text-xs font-normal text-gray-500">
        {changed} geändert
      </span>
    ) : null;

  const fields = (
    <div className="divide-y divide-gray-100">
      {visibleItems.map((setting) => (
        <SettingsField
          key={setting.key}
          setting={setting}
          categoryItems={category.items}
          highlighted={setting.key === highlightKey}
          onSave={onSave}
          onReset={onReset}
          onSchemaRefresh={onSchemaRefresh}
          onBookingAuthorityEnable={onBookingAuthorityEnable}
          audience={audience}
          revealFn={revealFn}
        />
      ))}
    </div>
  );

  if (!collapsible) {
    return (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur sm:p-6">
        {kicker && (
          <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            {kicker}
          </p>
        )}
        <h3 className="mb-1 flex flex-wrap items-center gap-2 text-base font-semibold text-gray-900">
          <span className="capitalize">{label}</span>
          {changedBadge}
        </h3>
        {fields}
      </div>
    );
  }

  return (
    <section className="moto-content-surface rounded-2xl border shadow-sm backdrop-blur">
      <h3 className="text-base font-semibold text-gray-900">
        <button
          type="button"
          aria-expanded={!isCollapsed}
          aria-controls={panelId}
          onClick={onToggle}
          className={cn(
            "flex w-full items-start gap-3 px-4 py-4 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:px-6",
            isCollapsed ? "rounded-2xl" : "rounded-t-2xl",
          )}
        >
          <ChevronDown
            className={cn(
              "mt-1 h-4 w-4 shrink-0 text-gray-400 transition-transform",
              isCollapsed && "-rotate-90",
            )}
            aria-hidden="true"
          />
          <span className="min-w-0 flex-1">
            <span className="flex flex-wrap items-center gap-2">
              <span className="capitalize">{label}</span>
              {changedBadge}
            </span>
            {isCollapsed && (
              <span className="mt-0.5 block truncate text-sm font-normal text-gray-500">
                {categorySummary(visibleItems)}
              </span>
            )}
          </span>
        </button>
      </h3>
      {!isCollapsed && (
        <div id={panelId} className="px-4 pb-4 sm:px-6 sm:pb-6">
          {fields}
        </div>
      )}
    </section>
  );
}
