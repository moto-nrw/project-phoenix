"use client";

import type { SchemaCategory } from "~/lib/settings-api";
import { SettingsField } from "./settings-field";

// Override labels for category keys that don't capitalize cleanly via CSS
// (acronyms read wrong when only the first letter is uppercased).
const categoryLabelOverrides: Record<string, string> = {
  mfa: "Zwei-Faktor-Authentifizierung",
  pin: "PIN",
};

function displayCategoryLabel(category: SchemaCategory): string {
  return categoryLabelOverrides[category.key] ?? category.label;
}

interface SettingsCategoryProps {
  readonly category: SchemaCategory;
  readonly highlightKey?: string | null;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
  // audience identifies who is viewing the settings page. Controls the
  // "auch von {other side} änderbar" hint on shared settings. Defaults
  // to "admin" for the tenant settings page; the operator page passes
  // "operator" to flip the hint copy.
  readonly audience?: "admin" | "operator";
  // revealFn is forwarded to PasswordField. Defaults to the tenant reveal
  // endpoint when unset. The operator page passes a school-bound function
  // so password reveal hits the operator endpoint instead.
  readonly revealFn?: (key: string) => Promise<string | null>;
}

export function SettingsCategory({
  category,
  highlightKey,
  onSave,
  onReset,
  audience = "admin",
  revealFn,
}: SettingsCategoryProps) {
  const visibleItems = category.items.filter((item) => item.visible);

  if (visibleItems.length === 0) {
    return null;
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm backdrop-blur sm:p-6">
      <h3 className="mb-1 text-base font-semibold text-gray-900 capitalize">
        {displayCategoryLabel(category)}
      </h3>
      <div className="divide-y divide-gray-100">
        {visibleItems.map((setting) => (
          <SettingsField
            key={setting.key}
            setting={setting}
            highlighted={setting.key === highlightKey}
            onSave={onSave}
            onReset={onReset}
            audience={audience}
            revealFn={revealFn}
          />
        ))}
      </div>
    </div>
  );
}
