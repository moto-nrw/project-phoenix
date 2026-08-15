"use client";

import Image from "next/image";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import { parentPath } from "~/lib/parent-url";
import { useShellAuth } from "~/lib/shell-auth-context";
import { UserCircle } from "./parent-icons";
import { isParentNavActive } from "./parent-nav-active";
import {
  PARENT_ICON_WEIGHT,
  PARENT_MORE_NAV,
  PARENT_PRIMARY_NAV,
} from "./parent-nav-items";

/**
 * Der Seitentitel kommt aus der einen Navigationsliste. Damit steht der Titel
 * genau einmal auf dem Bildschirm: hier oben, nicht noch einmal im Inhalt.
 */
function usePageTitleKey(pathname: string): string {
  const links = [
    ...PARENT_PRIMARY_NAV,
    ...PARENT_MORE_NAV.filter((item) => item.kind === "link"),
  ];
  // Laengster Treffer zuerst, sonst gewinnt /parents gegen /parents/children.
  const match = [...links]
    .sort((a, b) => b.href.length - a.href.length)
    .find((item) => isParentNavActive(item.href, pathname));
  return match?.tKey ?? "start";
}

/**
 * Kopfzeile der Eltern-App: Wortmarke, Seitentitel, Sprache und Konto.
 *
 * Die Wortmarke stammt aus dem Website-Repo (public/moto-logo-wordmark.webp);
 * die Website ist die einzige Designquelle.
 */
export function ParentHeader() {
  const t = useTranslations("parentNav");
  const pathname = usePathname();
  const { homeUrl, profileUrl } = useShellAuth();
  const titleKey = usePageTitleKey(pathname);

  return (
    <header className="sticky top-0 z-40 h-16 border-b border-gray-200 bg-white">
      <div className="flex h-16 items-center gap-3 px-4 sm:px-6">
        <Link
          href={homeUrl}
          aria-label="moto"
          className="flex shrink-0 items-center"
        >
          <Image
            src="/moto-logo-wordmark.webp"
            alt="moto"
            width={120}
            height={32}
            className="h-7 w-auto object-contain"
            priority
          />
        </Link>

        <span aria-hidden="true" className="h-6 w-px shrink-0 bg-gray-200" />

        {/* Kein h1: die Kopfzeile sagt, wo man ist, die Seite selbst traegt
            ihre Ueberschrift. Zwei h1 auf einer Seite waeren fuer die
            Sprachausgabe zwei Titel. */}
        <p className="min-w-0 flex-1 truncate text-[17px] font-semibold text-gray-900">
          {t(titleKey)}
        </p>

        <LanguageSwitcher compact />

        <Link
          href={profileUrl ?? parentPath("/parents/settings")}
          aria-label={t("settings")}
          className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl text-gray-600 hover:bg-gray-100 hover:text-gray-900"
        >
          <UserCircle
            size={26}
            weight={PARENT_ICON_WEIGHT}
            aria-hidden="true"
          />
        </Link>
      </div>
    </header>
  );
}
