"use client";

import { Languages } from "lucide-react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import {
  ListboxChevron,
  ListboxDropdown,
} from "~/components/ui/listbox-dropdown";
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
      size="dense"
      className={compact ? "max-w-[11rem]" : undefined}
      menuAlign="end"
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
          <ListboxChevron open={open} />
        </>
      )}
    />
  );
}
