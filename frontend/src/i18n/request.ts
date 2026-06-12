import { cookies, headers } from "next/headers";
import { getRequestConfig } from "next-intl/server";
import {
  DEFAULT_LOCALE,
  LOCALE_COOKIE_NAME,
  LOCALE_SCOPE_HEADER,
  type AppLocale,
} from "./locales";
import {
  mergeMessages,
  resolveLocale,
  type Messages,
} from "./locale-resolution";

async function loadMessages(locale: AppLocale): Promise<Messages> {
  const fallback = (await import("./messages/de.json")).default as Messages;
  if (locale === DEFAULT_LOCALE) return fallback;
  const selected = (await import(`./messages/${locale}.json`))
    .default as Messages;
  return mergeMessages(fallback, selected);
}

export default getRequestConfig(async () => {
  const cookieStore = await cookies();
  const headerStore = await headers();

  // Only parent-facing surfaces are localized. The proxy sets this header on
  // the parents subdomain and on public enrollment; everywhere else (the
  // German-only staff + operator portals) stays on the default locale so the
  // <html lang> attribute and any shared component never drift to a parent's
  // language or the browser's Accept-Language.
  const locale = resolveLocale({
    localized: headerStore.get(LOCALE_SCOPE_HEADER) === "1",
    cookieValue: cookieStore.get(LOCALE_COOKIE_NAME)?.value,
    acceptLanguage: headerStore.get("accept-language"),
  });

  return {
    locale,
    messages: await loadMessages(locale),
  };
});
