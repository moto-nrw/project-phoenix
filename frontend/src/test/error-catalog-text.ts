import { ERROR_CATALOG } from "~/lib/error-catalog.generated";
import {
  ERROR_CODE_CLASSES,
  type ErrorCode,
} from "~/lib/error-codes.generated";

/**
 * The German sentence the shared error path shows for `code` and `object`
 * (#2513): the code's own text, or its class text when the catalog has none.
 * Tests assert against it instead of copying catalog text, so a reworded
 * catalog entry does not break them.
 */
export function catalogText(code: ErrorCode, object: string): string {
  const de = ERROR_CATALOG.de as {
    classes: Record<string, string>;
    codes: Partial<Record<ErrorCode, string>>;
  };
  const template = de.codes[code] ?? de.classes[ERROR_CODE_CLASSES[code]]!;
  const text = template.replaceAll("{object}", object);
  return text.charAt(0).toUpperCase() + text.slice(1);
}
