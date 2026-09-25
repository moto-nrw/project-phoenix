import type { AppLocale } from "~/i18n/locales";
import { ApiError, errorClassCode } from "~/lib/api-error";
import { ERROR_CATALOG } from "~/lib/error-catalog.generated";
import {
  ERROR_CODES,
  ERROR_CODE_CLASSES,
  ERROR_CODE_PARAMETERS,
  type ErrorClass,
  type ErrorCode,
} from "~/lib/error-codes.generated";

const catalogs: Record<
  AppLocale,
  {
    classes: Record<ErrorClass, string>;
    actions: Record<keyof typeof ERROR_CATALOG.de.actions, string>;
    codes: Partial<Record<ErrorCode, string>>;
  }
> = ERROR_CATALOG;

const knownErrorCodes: ReadonlySet<string> = new Set(ERROR_CODES);

export interface ErrorPresentation {
  kind: "api" | "crash";
  errorClass?: ErrorClass;
  message: string;
  retryable: boolean;
  requestId?: string;
  fields: readonly string[];
  requiresLogin: boolean;
}

function isKnownErrorCode(code: string): code is ErrorCode {
  return knownErrorCodes.has(code);
}

function interpolate(
  template: string,
  object: string,
  parameters: Record<string, unknown>,
): string | null {
  let missing = false;
  const result = template.replaceAll(/\{([^{}]+)\}/g, (_, key: string) => {
    const value = key === "object" ? object : parameters[key];
    if (typeof value !== "string" && typeof value !== "number") {
      missing = true;
      return "";
    }
    return String(value);
  });
  return missing ? null : result.charAt(0).toUpperCase() + result.slice(1);
}

/** No backend sentence is read here. Only the code and structured fields are used. */
export function presentError(
  error: unknown,
  object: string,
  locale: AppLocale = "de",
): ErrorPresentation {
  const catalog = catalogs[locale];
  if (!(error instanceof ApiError)) {
    return {
      kind: "crash",
      message:
        interpolate(catalog.actions.crash, object, {}) ?? catalog.actions.crash,
      retryable: false,
      fields: [],
      requiresLogin: false,
    };
  }

  const code = error.code;
  const known = code && isKnownErrorCode(code) ? code : undefined;
  const errorClass: ErrorClass = known
    ? ERROR_CODE_CLASSES[known]
    : isKnownErrorCode(errorClassCode(error.status ?? 500))
      ? ERROR_CODE_CLASSES[errorClassCode(error.status ?? 500) as ErrorCode]
      : "server";
  const codeTemplate = known ? catalog.codes[known] : undefined;
  const allowed = new Set(known ? ERROR_CODE_PARAMETERS[known] : []);
  const values = Object.fromEntries(
    Object.entries(error.details ?? {}).filter(([key]) => allowed.has(key)),
  );
  const fallbackCatalog = known ? catalog : catalogs.de;
  const message =
    (codeTemplate && interpolate(codeTemplate, object, values)) ||
    interpolate(fallbackCatalog.classes[errorClass], object, {}) ||
    catalogs.de.classes.server;

  return {
    kind: "api",
    errorClass,
    message,
    retryable: errorClass === "unavailable" || errorClass === "server",
    requestId:
      errorClass === "unavailable" || errorClass === "server"
        ? error.requestId
        : undefined,
    fields: [...new Set(error.errors?.map((item) => item.field) ?? [])],
    requiresLogin: error.status === 401,
  };
}

export function errorDisplayLabels(locale: AppLocale) {
  return catalogs[locale].actions;
}
