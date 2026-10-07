import type { AppLocale } from "~/i18n/locales";
import { ERROR_CATALOG } from "~/lib/error-catalog.generated";
import {
  ERROR_CODE_CLASSES,
  type ErrorCode,
} from "~/lib/error-codes.generated";

/**
 * The sentence the shared error path shows for `code` and `object` (#2513):
 * the code's own text, or its class text when the catalog has none. German
 * unless a `locale` is given (#2518, parents portal). Tests assert against it
 * instead of copying catalog text, so a reworded catalog entry does not break
 * them.
 */
export function catalogText(
  code: ErrorCode,
  object: string,
  locale: AppLocale = "de",
): string {
  const catalog = ERROR_CATALOG[locale] as {
    classes: Record<string, string>;
    codes: Partial<Record<ErrorCode, string>>;
  };
  const template =
    catalog.codes[code] ?? catalog.classes[ERROR_CODE_CLASSES[code]]!;
  const text = template.replaceAll("{object}", object);
  return text.charAt(0).toUpperCase() + text.slice(1);
}
