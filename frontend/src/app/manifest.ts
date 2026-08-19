import type { MetadataRoute } from "next";
import { cookies, headers } from "next/headers";
import {
  faviconManifest,
  isParentsHost,
  resolveFaviconVariant,
} from "~/lib/favicon-variants";
import { resolveLocale } from "~/i18n/locale-resolution";
import { LOCALE_COOKIE_NAME } from "~/i18n/locales";
import { getParentAppMetadata } from "~/i18n/parent-app-metadata";

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set.`);
  return value;
}

export default async function manifest(): Promise<MetadataRoute.Manifest> {
  const requestHeaders = await headers();
  const host =
    requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "";
  const config = {
    operatorHostname: requiredEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME"),
    parentsHostname: requiredEnv("NEXT_PUBLIC_PARENTS_HOSTNAME"),
    tenantDomain: requiredEnv("TENANT_DOMAIN"),
  };

  const base = faviconManifest(resolveFaviconVariant(host, config));

  if (isParentsHost(host, config)) {
    const cookieStore = await cookies();
    const locale = resolveLocale({
      localized: true,
      cookieValue: cookieStore.get(LOCALE_COOKIE_NAME)?.value,
      acceptLanguage: requestHeaders.get("accept-language"),
    });
    const copy = getParentAppMetadata(locale);
    return {
      ...base,
      name: copy.name,
      short_name: "moto",
      description: copy.description,
      orientation: "portrait-primary",
      lang: locale,
    };
  }

  return base;
}
