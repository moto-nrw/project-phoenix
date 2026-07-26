"use client";

import type { ResolvedSetting, SchemaCategory } from "~/lib/settings-api";
import { SettingsField } from "./settings-field";

// Override labels for category keys that don't capitalize cleanly via CSS
// (acronyms read wrong when only the first letter is uppercased).
const categoryLabelOverrides: Record<string, string> = {
  mfa: "Zwei-Faktor-Authentifizierung",
  pin: "PIN",
  aktivitaeten: "Aktivitäten",
};

function displayCategoryLabel(category: SchemaCategory): string {
  return categoryLabelOverrides[category.key] ?? category.label;
}

const ENROLLMENT_LEGAL_TEXT_TO_TOGGLE_KEY: Record<string, string> = {
  "enrollment.legal_agb_text": "enrollment.legal_terms_enabled",
  "enrollment.legal_dsgvo_text": "enrollment.legal_dsgvo_enabled",
  "enrollment.legal_photo_text": "enrollment.legal_photo_enabled",
  "enrollment.legal_email_contact_text":
    "enrollment.legal_email_contact_enabled",
};

const HIDDEN_COMPANION_SETTINGS = new Set([
  "enrollment.legal_agb_document_url",
  "enrollment.legal_agb_display_mode",
]);

function shouldShowCategoryItem(
  item: ResolvedSetting,
  items: ResolvedSetting[],
): boolean {
  if (!item.visible) return false;
  if (HIDDEN_COMPANION_SETTINGS.has(item.key)) return false;
  if (item.key === "enrollment.legal_agb_text") return true;
  const toggleKey = ENROLLMENT_LEGAL_TEXT_TO_TOGGLE_KEY[item.key];
  if (!toggleKey) return true;
  const toggle = items.find((candidate) => candidate.key === toggleKey);
  return toggle?.value === true;
}

interface SettingsCategoryProps {
  readonly category: SchemaCategory;
  readonly highlightKey?: string | null;
  readonly onSave: (key: string, value: unknown) => Promise<string | null>;
  readonly onReset: (key: string) => Promise<string | null>;
  readonly onSchemaRefresh?: () => void;
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
  onSchemaRefresh,
  audience = "admin",
  revealFn,
}: SettingsCategoryProps) {
  const visibleItems = category.items.filter((item) =>
    shouldShowCategoryItem(item, category.items),
  );

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
            categoryItems={category.items}
            highlighted={setting.key === highlightKey}
            onSave={onSave}
            onReset={onReset}
            onSchemaRefresh={onSchemaRefresh}
            audience={audience}
            revealFn={revealFn}
          />
        ))}
      </div>
    </div>
  );
}
