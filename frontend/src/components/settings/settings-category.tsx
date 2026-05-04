"use client";

import type { SchemaCategory } from "~/lib/settings-api";
import { SettingsField } from "./settings-field";

interface SettingsCategoryProps {
  readonly category: SchemaCategory;
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
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <h3 className="mb-1 text-base font-semibold text-gray-900 capitalize">
        {category.label}
      </h3>
      <div className="divide-y divide-gray-100">
        {visibleItems.map((setting) => (
          <SettingsField
            key={setting.key}
            setting={setting}
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
