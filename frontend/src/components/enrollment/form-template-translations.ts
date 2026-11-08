/**
 * The texts of a form template that parents read, as targets of the
 * translations block, and the way back from an edited target to the object
 * that owns the text.
 */
import type { TranslationTarget } from "~/components/enrollment/translations-section";
import type {
  FormField,
  FormLegalBlock,
} from "~/lib/enrollment-form-schema-api";
import type { Translations } from "~/lib/enrollment-translations";
import {
  LEGAL_BLOCK_DISPLAY_MODE_PDF,
  legalBlockDisplayMode,
} from "~/lib/enrollment-legal-blocks";
import { copyStableObjectKey } from "~/lib/stable-object-key";

/** The owner of a translated text, by its position in the draft. */
type TranslationOwner =
  | { readonly kind: "legal"; readonly index: number }
  | { readonly kind: "field"; readonly index: number }
  | {
      readonly kind: "option";
      readonly index: number;
      readonly optionIndex: number;
    };

// The id is `legal|0|title`, `field|3|label` or `option|3|1|label`: the owner
// first, the attribute last. encode and decode below are the only places
// that know this format.
function encodeTranslationId(owner: TranslationOwner, attr: string): string {
  const position =
    owner.kind === "option"
      ? `${owner.index}|${owner.optionIndex}`
      : `${owner.index}`;
  return `${owner.kind}|${position}|${attr}`;
}

function decodeTranslationId(id: string): TranslationOwner | null {
  const [kind, first, second] = id.split("|");
  const index = Number(first);
  if (!Number.isInteger(index)) return null;
  if (kind === "legal" || kind === "field") return { kind, index };
  if (kind === "option") {
    const optionIndex = Number(second);
    return Number.isInteger(optionIndex) ? { kind, index, optionIndex } : null;
  }
  return null;
}

/** Every text of the template that parents read, in form order. */
export function schemaTranslationTargets(
  fields: FormField[],
  legalBlocks: FormLegalBlock[],
): TranslationTarget[] {
  const targets: TranslationTarget[] = [];
  const add = (
    owner: TranslationOwner,
    target: Omit<TranslationTarget, "id">,
  ) => {
    if (target.german.trim() !== "") {
      targets.push({ id: encodeTranslationId(owner, target.attr), ...target });
    }
  };
  legalBlocks.forEach((block, index) => {
    if (!block.enabled) return;
    const owner = { kind: "legal", index } as const;
    const { translations } = block;
    add(owner, {
      caption: "Zustimmung: Titel",
      german: block.title,
      attr: "title",
      translations,
    });
    add(owner, {
      caption: "Zustimmung: Text neben der Checkbox",
      german: block.label,
      attr: "label",
      translations,
    });
    // A PDF block shows a link to the file instead of its text.
    if (legalBlockDisplayMode(block) !== LEGAL_BLOCK_DISPLAY_MODE_PDF) {
      add(owner, {
        caption: "Zustimmung: Rechtstext / Erklärung",
        german: block.text,
        attr: "text",
        translations,
        multiline: true,
      });
    }
  });
  fields.forEach((field, index) => {
    const owner = { kind: "field", index } as const;
    const { translations } = field;
    const isInfo = field.type === "information";
    add(owner, {
      caption: isInfo ? "Infotext: Überschrift" : "Frage",
      german: field.label,
      attr: "label",
      translations,
    });
    add(owner, {
      caption: "Infotext: Inhalt",
      german: isInfo ? (field.content ?? "") : "",
      attr: "content",
      translations,
      multiline: true,
    });
    add(owner, {
      caption: "Hinweis zur Frage",
      german: isInfo ? "" : (field.help_text ?? ""),
      attr: "help_text",
      translations,
    });
    field.options?.forEach((option, optionIndex) =>
      add(
        { kind: "option", index, optionIndex },
        {
          caption: `Antwort zu „${field.label}“`,
          german: option.label,
          attr: "label",
          translations: option.translations,
        },
      ),
    );
  });
  return targets;
}

/**
 * The state update that writes edited translations back to the object that
 * owns the target's text: one of the two lists changes. An unknown id
 * changes nothing.
 */
export function schemaTranslationUpdate(
  targetId: string,
  translations: Translations,
): {
  readonly legalBlocks?: (blocks: FormLegalBlock[]) => FormLegalBlock[];
  readonly fields?: (fields: FormField[]) => FormField[];
} {
  const owner = decodeTranslationId(targetId);
  if (!owner) return {};
  if (owner.kind === "legal") {
    return {
      legalBlocks: (blocks) =>
        blocks.map((block, i) =>
          i === owner.index
            ? copyStableObjectKey(block, { ...block, translations })
            : block,
        ),
    };
  }
  return {
    fields: (fields) =>
      fields.map((field, i) => {
        if (i !== owner.index) return field;
        if (owner.kind === "field") {
          return copyStableObjectKey(field, { ...field, translations });
        }
        return copyStableObjectKey(field, {
          ...field,
          options: field.options?.map((option, optionIndex) =>
            optionIndex === owner.optionIndex
              ? { ...option, translations }
              : option,
          ),
        });
      }),
  };
}
