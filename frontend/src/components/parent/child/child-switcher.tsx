"use client";

import Link from "~/components/ui/navigation-link";
import { ChevronRight } from "lucide-react";
import { useTranslations } from "next-intl";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { ChildToday } from "~/lib/parent-api";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { parentPath } from "~/lib/parent-url";

/**
 * Die Auswahlseite fuer mehrere Kinder.
 *
 * Jedes Kind hat eine eigene Adresse und eine vollstaendige Identitaetskarte.
 * Nach der Auswahl verschwindet diese Liste, damit im Detail nur der Name des
 * gerade bearbeiteten Kindes den Kontext bestimmt.
 */

export interface ChildSwitcherItem {
  readonly studentId: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass?: string;
  readonly today: ChildToday;
}

function statusConcept(today: ChildToday): MotoConceptKey {
  if (today.at_ogs === null) return "unknown";
  return today.at_ogs ? "present" : "home";
}

export function ChildSwitcher({
  items,
}: Readonly<{
  items: readonly ChildSwitcherItem[];
}>) {
  const t = useTranslations("parentToday");
  if (items.length < 2) return null;

  return (
    <ul
      data-testid="child-selection"
      className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))] gap-4"
    >
      {items.map((item) => {
        const fullName = `${item.firstName} ${item.lastName}`;
        return (
          <li key={item.studentId}>
            <Link
              href={parentPath(`/parents/children/${item.studentId}`)}
              className="moto-content-surface group flex min-h-40 flex-col rounded-2xl border p-4 shadow-sm transition-[border-color,box-shadow] hover:border-gray-300 hover:shadow-md focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:p-5"
            >
              <span className="flex w-full min-w-0 items-start justify-between gap-4">
                <span className="flex min-w-0 items-center gap-3">
                  <span className="grid size-11 shrink-0 place-items-center rounded-xl bg-gray-50">
                    <MotoConceptIcon
                      concept="children"
                      size={28}
                      aria-hidden="true"
                    />
                  </span>
                  <span className="min-w-0">
                    <span className="block text-xl leading-snug font-semibold tracking-tight text-balance break-words text-gray-900">
                      {fullName}
                    </span>
                    {item.schoolClass && (
                      <span className="mt-1 block text-sm text-gray-600">
                        {item.schoolClass}
                      </span>
                    )}
                  </span>
                </span>
                <span className="inline-flex shrink-0 items-center gap-1 text-sm font-medium text-gray-600 group-hover:text-gray-900">
                  {t("actions.profile")}
                  <ChevronRight className="size-4" aria-hidden="true" />
                </span>
              </span>

              <span className="mt-5 flex w-full min-w-0 items-center gap-3 rounded-xl bg-gray-50 p-3">
                <MotoConceptIcon
                  concept={statusConcept(item.today)}
                  size={26}
                  aria-hidden="true"
                />
                <span className="min-w-0">
                  <span className="block text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    {t("today")}
                  </span>
                  <span className="mt-0.5 block text-sm font-medium text-gray-900">
                    {item.today.at_ogs === null
                      ? t("state.unknown")
                      : item.today.at_ogs
                        ? t("atOgs")
                        : t("notAtOgs")}
                  </span>
                </span>
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
