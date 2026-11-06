"use client";

import type { ReactNode } from "react";

import { StatCard } from "~/components/ui/stat-card";
import { BackLink } from "~/components/ui/back-link";
import { DataTableStatusBadge } from "~/components/ui/data-table";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import type { MotoConcept } from "~/lib/moto-concepts";

interface EntityHeaderStat {
  label: string;
  value: string | number;
  /** Färbt die Zahl, wenn sie mehr sagt als ihre Höhe (Kontingent überschritten). */
  tone?: "neutral" | "orange";
  /** Erklärt den Rechenweg als Titel-Hinweis. */
  title?: string;
}

interface EntityHeaderCardProps {
  title: string;
  /**
   * Optional fachliches Konzept fuer den Titel. Wenn gesetzt, rendert eine
   * graue Icon-Kachel links vom Titel (Header-Muster). Ohne Prop
   * unveraendertes Verhalten.
   */
  concept?: MotoConcept;
  subdomain?: string | null;
  active: boolean;
  stats?: EntityHeaderStat[];
  createdAt?: string | Date | null;
  actions?: ReactNode;
  subtitle?: ReactNode;
  activeLabel?: string;
  inactiveLabel?: string;
  /** Rückweg zur übergeordneten Seite, über dem Titel. */
  back?: { href: string; label: string };
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
  back,
}: Readonly<EntityHeaderCardProps>) {
  const year = createdAt ? formatYear(createdAt) : null;
  const combinedStats: EntityHeaderStat[] = [...(stats ?? [])];
  if (year) {
    combinedStats.push({ label: "Seit", value: year });
  }

  return (
    <section className="moto-content-surface rounded-xl border p-6">
      {back ? (
        <BackLink href={back.href} label={back.label} className="mb-2" />
      ) : null}
      <div className="flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 flex-1 items-start gap-3 sm:gap-4">
          {concept ? <ConceptIconTile concept={concept} size="page" /> : null}
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
        <div className="mt-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          {combinedStats.map((stat) => (
            <StatCard
              key={stat.label}
              label={stat.label}
              value={stat.value}
              tone={stat.tone ?? "neutral"}
              title={stat.title}
            />
          ))}
        </div>
      ) : null}
    </section>
  );
}
