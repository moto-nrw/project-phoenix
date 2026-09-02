"use client";

import type { SchemaCategory } from "~/lib/settings-api";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
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
   * Renders the category as a collapsible `SectionCard` (#2830). The open
   * state is controlled by the parent via `collapsed` + `onToggle`, so a tab
   * can expand the category that holds a deep-linked setting or expand every
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
  const visibleItems = filterCategoryItems(category, filterQuery);

  if (visibleItems.length === 0) {
    return null;
  }

  const isCollapsed = collapsible && collapsed;
  const changed = changedCount(visibleItems);

  return (
    <SectionCard
      headingLevel={3}
      kicker={kicker}
      title={displayCategoryLabel(category)}
      titleClassName="capitalize"
      titleBadge={
        changed > 0 ? (
          <StatusBadge
            tone="gray"
            showDot={false}
            label={`${changed} geändert`}
          />
        ) : undefined
      }
      // While collapsed the card names what it holds, so a person can tell
      // from the closed header whether the setting they need lives here.
      description={isCollapsed ? categorySummary(visibleItems) : undefined}
      collapsible={collapsible}
      collapsed={collapsible ? collapsed : undefined}
      onCollapsedChange={onToggle}
    >
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
    </SectionCard>
  );
}
