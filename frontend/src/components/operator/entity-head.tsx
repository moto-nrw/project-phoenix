"use client";

import type { ReactNode } from "react";

import { DataTableStatusBadge } from "~/components/ui/data-table";
import { StatCard } from "~/components/ui/stat-card";

interface EntityStat {
  label: string;
  value: string | number;
  /** Färbt die Zahl, wenn sie mehr sagt als ihre Höhe (Kontingent überschritten). */
  tone?: "neutral" | "orange";
  /** Erklärt den Rechenweg als Titel-Hinweis. */
  title?: string;
}

function formatYear(value: string | Date): string | null {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return String(date.getFullYear());
}

/**
 * Statuszeile einer Träger- oder Schulseite im Operator-Portal: Adresse,
 * aktiv oder inaktiv, darunter ein Zusatz. Sie steht in `TenantPage.stats`.
 */
export function EntityStatusLine({
  subdomain,
  active,
  activeLabel,
  inactiveLabel,
  subtitle,
}: Readonly<{
  subdomain?: string | null;
  active: boolean;
  activeLabel?: string;
  inactiveLabel?: string;
  subtitle?: ReactNode;
}>) {
  return (
    <>
      <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
        {subdomain ? (
          <span className="font-mono text-gray-500">{subdomain}</span>
        ) : null}
        {subdomain ? <span className="text-gray-300">·</span> : null}
        <DataTableStatusBadge
          active={active}
          activeLabel={activeLabel}
          inactiveLabel={inactiveLabel}
        />
      </span>
      {subtitle ? <span className="mt-2 block">{subtitle}</span> : null}
    </>
  );
}

/** Kennzahlen einer Träger- oder Schulseite, auf ihrem Reiter „Übersicht“. */
export function EntityStats({
  stats,
  createdAt,
}: Readonly<{
  stats?: EntityStat[];
  createdAt?: string | Date | null;
}>) {
  const year = createdAt ? formatYear(createdAt) : null;
  const combined: EntityStat[] = [...(stats ?? [])];
  if (year) {
    combined.push({ label: "Seit", value: year });
  }
  if (combined.length === 0) return null;

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
      {combined.map((stat) => (
        <StatCard
          key={stat.label}
          label={stat.label}
          value={stat.value}
          tone={stat.tone ?? "neutral"}
          title={stat.title}
        />
      ))}
    </div>
  );
}
