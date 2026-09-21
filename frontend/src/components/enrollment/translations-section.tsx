"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { DEFAULT_LOCALE, SUPPORTED_LOCALES } from "~/i18n/locales";
import {
  translationStatus,
  withTranslation,
  type Translations,
  type TranslationStatus,
} from "~/lib/enrollment-translations";

/** One German text the school wrote, with the document that translates it. */
export interface TranslationTarget {
  /** Stable per text, e.g. `field:allergies:label`. */
  readonly id: string;
  /** What the text is, in the editor's own words ("Frage", "Hinweis"). */
  readonly caption: string;
  readonly german: string;
  readonly attr: string;
  readonly translations?: Translations | null;
  readonly multiline?: boolean;
}

const TARGET_LOCALES = SUPPORTED_LOCALES.filter(
  (locale) => locale.code !== DEFAULT_LOCALE,
).map((locale) => ({ value: locale.code as string, label: locale.label }));

const STATUS_BADGE: Record<
  TranslationStatus,
  { label: string; tone: "gray" | "green" | "orange" }
> = {
  missing: { label: "Fehlt", tone: "gray" },
  current: { label: "Fertig", tone: "green" },
  stale: { label: "Bitte prüfen", tone: "orange" },
};

/**
 * Editor block for the school-written translations of one object (#3377).
 * It lists every German text next to its translation in the chosen language
 * and reports the next translation document through `onChange`; the
 * surrounding editor saves it together with the object.
 */
export function TranslationsSection({
  targets,
  onChange,
  disabled = false,
}: {
  readonly targets: readonly TranslationTarget[];
  readonly onChange: (target: TranslationTarget, next: Translations) => void;
  readonly disabled?: boolean;
}) {
  const [locale, setLocale] = useState(TARGET_LOCALES[0]?.value ?? "");
  const filled = targets.filter(
    (target) =>
      translationStatus(
        target.translations?.[locale]?.[target.attr],
        target.german,
      ) === "current",
  ).length;

  return (
    <section className="space-y-4 border-t border-gray-100 pt-5">
      <div>
        <h2 className="text-base font-semibold text-gray-900">
          Übersetzungen für Eltern
        </h2>
        <p className="mt-1 max-w-2xl text-sm text-gray-600">
          Eltern wählen im Formular ihre Sprache. Ohne Übersetzung lesen sie den
          deutschen Text.
        </p>
      </div>

      {targets.length === 0 ? (
        <p className="text-sm text-gray-600">
          Hier gibt es noch keine eigenen Texte zum Übersetzen.
        </p>
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <SegmentedControl
              items={TARGET_LOCALES}
              value={locale}
              onChange={setLocale}
              ariaLabel="Sprache der Übersetzung"
            />
            <p className="text-sm text-gray-600">
              {filled} von {targets.length} übersetzt
            </p>
          </div>
          <ul className="space-y-3">
            {targets.map((target) => (
              <TranslationRow
                key={target.id}
                target={target}
                locale={locale}
                disabled={disabled}
                onChange={onChange}
              />
            ))}
          </ul>
        </>
      )}
    </section>
  );
}

function TranslationRow({
  target,
  locale,
  disabled,
  onChange,
}: {
  readonly target: TranslationTarget;
  readonly locale: string;
  readonly disabled: boolean;
  readonly onChange: (target: TranslationTarget, next: Translations) => void;
}) {
  const entry = target.translations?.[locale]?.[target.attr];
  const status = translationStatus(entry, target.german);
  const badge = STATUS_BADGE[status];
  const fieldId = `translation-${target.id}-${locale}`;
  const setText = (text: string) =>
    onChange(
      target,
      withTranslation(
        target.translations,
        locale,
        target.attr,
        text,
        target.german,
      ),
    );
  const Field = target.multiline ? Textarea : Input;

  return (
    <li className="grid gap-3 rounded-xl border border-gray-200 p-3 sm:grid-cols-2">
      <div className="min-w-0">
        <p className="text-xs font-medium text-gray-500">{target.caption}</p>
        <p className="mt-1 text-sm break-words whitespace-pre-line text-gray-900">
          {target.german}
        </p>
      </div>
      <div className="min-w-0 space-y-2">
        <div className="flex items-center justify-between gap-2">
          <label
            htmlFor={fieldId}
            className="text-xs font-medium text-gray-500"
          >
            Übersetzung
          </label>
          <StatusBadge label={badge.label} tone={badge.tone} compact />
        </div>
        <Field
          id={fieldId}
          value={entry?.text ?? ""}
          rows={target.multiline ? 4 : undefined}
          disabled={disabled}
          onChange={(event) => setText(event.target.value)}
        />
        {status === "stale" && (
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs text-gray-600">
              Der deutsche Text wurde geändert. Eltern lesen bis zur Prüfung
              Deutsch.
            </p>
            <Button
              type="button"
              variant="outline"
              size="compact"
              disabled={disabled}
              onClick={() => setText(entry?.text ?? "")}
            >
              Passt noch
            </Button>
          </div>
        )}
      </div>
    </li>
  );
}
