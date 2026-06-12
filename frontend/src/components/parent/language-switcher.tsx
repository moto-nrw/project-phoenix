"use client";

import { Languages } from "lucide-react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import type { AppLocale } from "~/i18n/locales";
import {
  normalizeLocale,
  SUPPORTED_LOCALES,
  writeLocaleCookie,
} from "~/i18n/locales";
import { useParentLocale } from "~/lib/parent-locale-context";

export function LanguageSwitcher({
  compact = false,
}: Readonly<{ compact?: boolean }>) {
  const t = useTranslations("language");
  const router = useRouter();
  const intlLocale = normalizeLocale(useLocale());
  const context = useParentLocale();
  const current = context?.locale ?? intlLocale;
  const currentLabel =
    SUPPORTED_LOCALES.find((locale) => locale.code === current)?.label ??
    current;

  const changeLocale = (locale: AppLocale) => {
    if (context) {
      void context.setLocale(locale);
      return;
    }
    // Anonymous surfaces (public enrollment): no profile to persist to, so
    // just set the cookie and soft-refresh the server tree in the new locale.
    writeLocaleCookie(locale);
    router.refresh();
  };

  return (
    <div
      className={`relative inline-flex min-h-9 cursor-pointer items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:border-gray-300 ${
        compact ? "max-w-[11rem]" : ""
      }`}
    >
      <Languages
        className="h-4 w-4 shrink-0 text-gray-500"
        aria-hidden="true"
      />
      <span className={compact ? "sr-only" : "text-gray-600"}>
        {t("label")}
      </span>
      <span className="min-w-0 truncate text-sm font-semibold text-gray-900">
        {currentLabel}
      </span>
      <select
        value={current}
        onChange={(event) => changeLocale(event.target.value as AppLocale)}
        className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
        aria-label={t("label")}
      >
        {SUPPORTED_LOCALES.map((locale) => (
          <option key={locale.code} value={locale.code}>
            {locale.label}
          </option>
        ))}
      </select>
    </div>
  );
}
