// School-written translations of enrollment form texts (#3377).
//
// The German text is the attribute itself (`label`, `name`, …). Next to it an
// entity may carry `translations`: locale → attribute → { text, source }.
// `source` is the German text the translator saw. A translation only counts
// while `source` still equals the current German text; otherwise parents read
// German until the school confirms the translation again. Parent-facing
// responses arrive pre-filtered and without `source`; the admin preview
// carries the stored document, so the resolver checks `source` when present.

import { DEFAULT_LOCALE } from "~/i18n/locales";

export interface TranslatedText {
  text: string;
  source?: string;
}

export type Translations = Record<string, Record<string, TranslatedText>>;

interface Translatable {
  translations?: Translations | null;
}

export type TranslationStatus = "missing" | "current" | "stale";

/** State of one stored translation against the current German text. */
export function translationStatus(
  entry: TranslatedText | undefined,
  germanText: string,
): TranslationStatus {
  if (!entry || entry.text.trim() === "") return "missing";
  return (entry.source ?? "").trim() === germanText.trim()
    ? "current"
    : "stale";
}

/**
 * Document with `attr` in `locale` set to `text`, recorded against the German
 * text the translator sees right now. An empty text removes the entry; an
 * emptied document becomes `{}`, which clears the stored translations.
 */
export function withTranslation(
  translations: Translations | null | undefined,
  locale: string,
  attr: string,
  text: string,
  germanText: string,
): Translations {
  const next: Translations = { ...translations };
  const attrs = { ...next[locale] };
  if (text.trim() === "") {
    delete attrs[attr];
  } else {
    attrs[attr] = { text, source: germanText.trim() };
  }
  if (Object.keys(attrs).length === 0) {
    delete next[locale];
  } else {
    next[locale] = attrs;
  }
  return next;
}

/**
 * Text of `attr` in `locale`, falling back to the German text per attribute.
 */
export function translatedText(
  entity: Translatable,
  attr: string,
  germanText: string,
  locale: string,
): string {
  if (locale === DEFAULT_LOCALE) return germanText;
  const entry = entity.translations?.[locale]?.[attr];
  if (!entry || entry.text.trim() === "") return germanText;
  if (entry.source !== undefined && entry.source.trim() !== germanText.trim()) {
    return germanText;
  }
  return entry.text;
}

/**
 * Copy of `entity` whose `attrs` read in `locale`. Attributes that are empty
 * or not strings stay untouched, so optional fields keep their absence.
 */
function localizeEntity<T extends Translatable>(
  entity: T,
  attrs: readonly (keyof T & string)[],
  locale: string,
): T {
  if (locale === DEFAULT_LOCALE || !entity.translations) return entity;
  const copy = { ...entity };
  for (const attr of attrs) {
    const german = entity[attr];
    if (typeof german !== "string" || german === "") continue;
    copy[attr] = translatedText(entity, attr, german, locale) as T[typeof attr];
  }
  return copy;
}

// The shapes below are structural on purpose: this module stays free of
// imports from the enrollment API modules, which import its types.

interface TranslatableOption extends Translatable {
  label: string;
}

interface TranslatableField extends Translatable {
  label: string;
  help_text?: string;
  content?: string;
  options?: TranslatableOption[];
}

/** Form fields with label, help text, content and option labels in `locale`. */
export function localizeFormFields<F extends TranslatableField>(
  fields: F[],
  locale: string,
): F[] {
  if (locale === DEFAULT_LOCALE) return fields;
  return fields.map((field) => {
    const localized = localizeEntity<TranslatableField>(
      field,
      ["label", "help_text", "content"],
      locale,
    ) as F;
    if (!field.options?.length) return localized;
    return {
      ...localized,
      options: field.options.map((option) =>
        localizeEntity(option, ["label"], locale),
      ),
    };
  });
}

interface TranslatableLegalBlock extends Translatable {
  title: string;
  label: string;
  text: string;
}

/** Legal blocks with title, checkbox label and text in `locale`. */
export function localizeLegalBlocks<B extends TranslatableLegalBlock>(
  blocks: B[],
  locale: string,
): B[] {
  if (locale === DEFAULT_LOCALE) return blocks;
  return blocks.map(
    (block) =>
      localizeEntity<TranslatableLegalBlock>(
        block,
        ["title", "label", "text"],
        locale,
      ) as B,
  );
}

interface TranslatableOffering extends Translatable {
  name: string;
  description?: string | null;
}

/** Care offerings with name and description in `locale`. */
export function localizeOfferings<O extends TranslatableOffering>(
  offerings: O[],
  locale: string,
): O[] {
  if (locale === DEFAULT_LOCALE) return offerings;
  return offerings.map(
    (offering) =>
      localizeEntity<TranslatableOffering>(
        offering,
        ["name", "description"],
        locale,
      ) as O,
  );
}

interface TranslatableGrouped extends Translatable {
  selection_group?: string | null;
}

/**
 * Heading of a selection group in `locale`. The German group name stays the
 * key that ties offerings to one rule, so it is never rewritten on the
 * offering; only what parents read is translated. Offerings of one group may
 * be translated independently: the first one that carries a translation wins.
 */
export function selectionGroupLabel(
  offerings: readonly TranslatableGrouped[],
  group: string,
  locale: string,
): string {
  for (const offering of offerings) {
    if ((offering.selection_group?.trim() ?? "") !== group) continue;
    const label = translatedText(offering, "selection_group", group, locale);
    if (label !== group) return label;
  }
  return group;
}

interface TranslatableNamed extends Translatable {
  name: string;
}

/** A phase (or any named entity) with its name in `locale`. */
export function localizeNamed<N extends TranslatableNamed>(
  entity: N,
  locale: string,
): N {
  return localizeEntity<TranslatableNamed>(entity, ["name"], locale) as N;
}
