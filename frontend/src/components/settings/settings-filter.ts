import type {
  ResolvedSetting,
  SchemaCategory,
  SchemaTab,
} from "~/lib/settings-api";

// Override labels for category keys that don't capitalize cleanly via CSS
// (acronyms read wrong when only the first letter is uppercased).
const categoryLabelOverrides: Record<string, string> = {
  mfa: "Zwei-Faktor-Authentifizierung",
  pin: "PIN",
  aktivitaeten: "Aktivitäten",
  stundenplan: "Betreuungsplan",
};

export function displayCategoryLabel(category: SchemaCategory): string {
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

/**
 * Whether a setting is shown inside its category at all: hidden by
 * DependsOn, hidden companion settings (edited through another field), and
 * enrollment legal texts whose toggle is off.
 */
function shouldShowCategoryItem(
  item: ResolvedSetting,
  items: readonly ResolvedSetting[],
): boolean {
  if (!item.visible) return false;
  if (HIDDEN_COMPANION_SETTINGS.has(item.key)) return false;
  if (item.key === "enrollment.legal_agb_text") return true;
  const toggleKey = ENROLLMENT_LEGAL_TEXT_TO_TOGGLE_KEY[item.key];
  if (!toggleKey) return true;
  const toggle = items.find((candidate) => candidate.key === toggleKey);
  return toggle?.value === true;
}

export function visibleCategoryItems(
  category: SchemaCategory,
): ResolvedSetting[] {
  return category.items.filter((item) =>
    shouldShowCategoryItem(item, category.items),
  );
}

/** Lower-cases and trims a search query; "" means "no filter". */
export function normalizeQuery(query: string): string {
  return query.trim().toLowerCase();
}

function textMatches(text: string | undefined, query: string): boolean {
  return text != null && text.toLowerCase().includes(query);
}

function settingMatches(
  item: ResolvedSetting,
  normalizedQuery: string,
): boolean {
  if (normalizedQuery === "") return true;
  return (
    textMatches(item.label, normalizedQuery) ||
    textMatches(item.description, normalizedQuery)
  );
}

/**
 * The items of a category that match a query. A query that hits the
 * category name itself returns every visible item, so a person searching
 * "Zeiterfassung" sees the whole block. Without a query, all visible items.
 */
export function filterCategoryItems(
  category: SchemaCategory,
  normalizedQuery: string,
): ResolvedSetting[] {
  const visible = visibleCategoryItems(category);
  if (normalizedQuery === "") return visible;
  if (textMatches(displayCategoryLabel(category), normalizedQuery)) {
    return visible;
  }
  return visible.filter((item) => settingMatches(item, normalizedQuery));
}

export interface CategorySearchHit {
  readonly tab: SchemaTab;
  readonly category: SchemaCategory;
  readonly items: ResolvedSetting[];
}

/** Every category across the given tabs with at least one matching item. */
export function searchTabs(
  tabs: readonly SchemaTab[],
  normalizedQuery: string,
): CategorySearchHit[] {
  const hits: CategorySearchHit[] = [];
  for (const tab of tabs) {
    for (const category of tab.categories) {
      const items = filterCategoryItems(category, normalizedQuery);
      if (items.length > 0) hits.push({ tab, category, items });
    }
  }
  return hits;
}

const SUMMARY_LABEL_COUNT = 3;

/**
 * One line that says what a collapsed category contains, built from the
 * setting labels: "Krankmeldung, Nachrichten, Essensplan und 4 weitere".
 */
export function categorySummary(items: readonly ResolvedSetting[]): string {
  const labels = items.map((item) => item.label);
  if (labels.length <= SUMMARY_LABEL_COUNT) return labels.join(", ");
  const shown = labels.slice(0, SUMMARY_LABEL_COUNT).join(", ");
  const rest = labels.length - SUMMARY_LABEL_COUNT;
  return `${shown} und ${rest} weitere`;
}

/** How many settings in the list carry a value other than the default. */
export function changedCount(items: readonly ResolvedSetting[]): number {
  return items.filter((item) => !item.is_default).length;
}
