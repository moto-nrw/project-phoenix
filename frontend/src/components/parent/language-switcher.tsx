"use client";

import { ChevronDown, Languages } from "lucide-react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
import type { AppLocale } from "~/i18n/locales";
import {
  normalizeLocale,
  SUPPORTED_LOCALES,
  writeLocaleCookie,
} from "~/i18n/locales";
import { useParentLocale } from "~/lib/parent-locale-context";

const localeOptions = SUPPORTED_LOCALES.map((locale) => ({
  value: locale.code,
  label: locale.label,
}));

export function LanguageSwitcher({
  compact = false,
}: Readonly<{ compact?: boolean }>) {
  const t = useTranslations("language");
  const router = useRouter();
  const intlLocale = normalizeLocale(useLocale());
  const context = useParentLocale();
  const current = context?.locale ?? intlLocale;
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    setHydrated(true);
  }, []);

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
    <ListboxDropdown
      value={current}
      options={localeOptions}
      onChange={changeLocale}
      ariaLabel={t("label")}
      containerClassName="relative inline-block align-middle"
      containerStyle={{ visibility: hydrated ? "visible" : "hidden" }}
      className={`inline-flex h-9 cursor-pointer items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 text-sm leading-5 font-medium text-gray-700 shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        compact ? "max-w-[11rem]" : ""
      }`}
      menuAlign="end"
      menuClassName="moto-popover-surface overflow-y-auto rounded-xl border py-1"
      optionClassName="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
      activeOptionClassName="flex w-full items-center gap-2 bg-gray-50 px-4 py-2 text-left text-sm font-medium text-gray-900 transition-colors"
      disabledOptionClassName="flex w-full cursor-not-allowed items-center gap-2 px-4 py-2 text-left text-sm text-gray-400 transition-colors"
      renderTrigger={({ selectedLabel, open }) => (
        <>
          <Languages
            className="h-4 w-4 shrink-0 text-gray-500"
            aria-hidden="true"
          />
          <span className={compact ? "sr-only" : "text-gray-600"}>
            {t("label")}
          </span>
          <span
            className={`min-w-0 truncate text-sm font-semibold text-gray-900 ${
              compact ? "hidden md:inline" : ""
            }`}
          >
            {selectedLabel}
          </span>
          <ChevronDown
            aria-hidden="true"
            className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${
              open ? "rotate-180" : ""
            }`}
          />
        </>
      )}
    />
  );
}
