"use client";

import Link from "next/link";
import { UsersRound, Activity, UserCheck } from "lucide-react";
import { Card, CardContent } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import type { LucideIcon } from "lucide-react";

interface CompactStatsProps {
  readonly activeOGSGroups: number;
  readonly activeActivities: number;
  readonly supervisorsToday: number;
  readonly studentsPresent: number;
  readonly loading: boolean;
}

interface StatItemProps {
  readonly icon: LucideIcon;
  readonly label: string;
  readonly value: number;
  readonly subtitle?: string;
  readonly href: string;
  readonly loading: boolean;
}

function StatItem({
  icon: Icon,
  label,
  value,
  subtitle,
  href,
  loading,
}: StatItemProps) {
  return (
    <Link
      href={href}
      className="group block rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
    >
      <Card className="h-full transition-shadow group-hover:shadow-md">
        <CardContent className="p-3 md:p-4">
          {loading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-16" />
              <Skeleton className="h-7 w-10" />
            </div>
          ) : (
            <div className="flex items-start gap-2.5">
              <Icon className="text-muted-foreground mt-0.5 h-4 w-4 flex-shrink-0" />
              <div className="min-w-0">
                <p className="text-muted-foreground text-xs font-medium">
                  {label}
                </p>
                <p className="text-2xl font-bold tracking-tight">{value}</p>
                {subtitle && (
                  <p className="text-muted-foreground text-xs">{subtitle}</p>
                )}
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}

export function CompactStats({
  activeOGSGroups,
  activeActivities,
  supervisorsToday,
  studentsPresent,
  loading,
}: CompactStatsProps) {
  const showRatio = studentsPresent > 0 && supervisorsToday > 0;
  const ratio = showRatio
    ? Math.round(studentsPresent / supervisorsToday)
    : null;

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3">
      <StatItem
        icon={UsersRound}
        label="Aktive Gruppen"
        value={activeOGSGroups}
        href="/ogs-groups"
        loading={loading}
      />
      <StatItem
        icon={Activity}
        label="Aktivitäten"
        value={activeActivities}
        href="/activities"
        loading={loading}
      />
      <StatItem
        icon={UserCheck}
        label="Betreuer"
        value={supervisorsToday}
        subtitle={ratio !== null ? `${ratio}:1 Betreuungsschlüssel` : undefined}
        href="/staff"
        loading={loading}
      />
    </div>
  );
}
