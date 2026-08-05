"use client";

import type { ReactNode } from "react";

import { DataTableStatusBadge } from "~/components/ui/data-table";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";

interface EntityHeaderStat {
  label: string;
  value: ReactNode;
}

interface EntityHeaderCardProps {
  title: string;
  /**
   * Optional fachliches Konzept fuer den Titel. Wenn gesetzt, rendert eine
   * graue Icon-Kachel links vom Titel (Header-Muster). Ohne Prop
   * unveraendertes Verhalten.
   */
  concept?: MotoConceptKey;
  subdomain?: string | null;
  active: boolean;
  stats?: EntityHeaderStat[];
  createdAt?: string | Date | null;
  actions?: ReactNode;
  subtitle?: ReactNode;
  activeLabel?: string;
  inactiveLabel?: string;
}

function formatYear(value: string | Date): string | null {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return String(date.getFullYear());
}

export function EntityHeaderCard({
  title,
  concept,
  subdomain,
  active,
  stats,
  createdAt,
  actions,
  subtitle,
  activeLabel,
  inactiveLabel,
}: Readonly<EntityHeaderCardProps>) {
  const year = createdAt ? formatYear(createdAt) : null;
  const combinedStats: EntityHeaderStat[] = [...(stats ?? [])];
  if (year) {
    combinedStats.push({ label: "Seit", value: year });
  }

  return (
    <section className="rounded-xl border border-gray-200 bg-white p-6 shadow-sm">
      <div className="flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 flex-1 items-start gap-3 sm:gap-4">
          {concept ? (
            <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-12 sm:w-12">
              <MotoConceptIcon concept={concept} size={26} />
            </div>
          ) : null}
          <div className="min-w-0 flex-1">
            <h1 className="text-2xl font-bold text-gray-900">{title}</h1>
            <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
              {subdomain ? (
                <span className="font-mono text-gray-500">{subdomain}</span>
              ) : null}
              {subdomain ? <span className="text-gray-300">·</span> : null}
              <DataTableStatusBadge
                active={active}
                activeLabel={activeLabel}
                inactiveLabel={inactiveLabel}
              />
            </div>
            {subtitle ? (
              <div className="mt-2 text-sm text-gray-600">{subtitle}</div>
            ) : null}
          </div>
        </div>

        {actions ? (
          <div className="flex flex-wrap items-center gap-2">{actions}</div>
        ) : null}
      </div>

      {combinedStats.length > 0 ? (
        <div className="mt-6 flex flex-wrap gap-x-10 gap-y-4">
          {combinedStats.map((stat) => (
            <div key={stat.label} className="min-w-0">
              <div className="text-2xl font-bold text-gray-900">
                {stat.value}
              </div>
              <div className="text-xs tracking-wider text-gray-500 uppercase">
                {stat.label}
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </section>
  );
}
