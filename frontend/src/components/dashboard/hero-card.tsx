"use client";

import Link from "next/link";
import { Users } from "lucide-react";
import { Card, CardContent } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import { cn } from "~/lib/utils";

interface HeroCardProps {
  readonly studentsPresent: number;
  readonly studentsInRooms: number;
  readonly studentsInTransit: number;
  readonly studentsOnPlayground: number;
  readonly loading: boolean;
}

const SEGMENTS = [
  { key: "rooms", label: "In Räumen", color: "bg-blue-500" },
  { key: "transit", label: "Unterwegs", color: "bg-orange-500" },
  { key: "playground", label: "Schulhof", color: "bg-yellow-400" },
] as const;

export function HeroCard({
  studentsPresent,
  studentsInRooms,
  studentsInTransit,
  studentsOnPlayground,
  loading,
}: HeroCardProps) {
  const values: Record<string, number> = {
    rooms: studentsInRooms,
    transit: studentsInTransit,
    playground: studentsOnPlayground,
  };

  const hasStudents = studentsPresent > 0;

  return (
    <Link
      href="/students/search"
      className="group block rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
    >
      <Card className="transition-shadow group-hover:shadow-md">
        <CardContent className="p-4 md:p-6">
          {loading ? (
            <div className="space-y-4">
              <div className="flex items-center gap-3">
                <Skeleton className="h-10 w-10 rounded-lg" />
                <Skeleton className="h-10 w-32" />
              </div>
              <Skeleton className="h-3 w-full rounded-full" />
              <div className="flex gap-4">
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-24" />
                <Skeleton className="h-4 w-24" />
              </div>
            </div>
          ) : (
            <>
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-blue-50 text-blue-600">
                  <Users className="h-5 w-5" />
                </div>
                <div>
                  <p className="text-muted-foreground text-sm font-medium">
                    Kinder anwesend
                  </p>
                  <p className="text-4xl font-bold tracking-tight md:text-5xl">
                    {studentsPresent}
                  </p>
                </div>
              </div>

              {hasStudents && (
                <div className="mt-4 space-y-2">
                  {/* Breakdown bar */}
                  <div className="flex h-3 overflow-hidden rounded-full bg-gray-100">
                    {SEGMENTS.map(({ key, color }) => {
                      const value = values[key] ?? 0;
                      if (value === 0) return null;
                      const pct = Math.max((value / studentsPresent) * 100, 4);
                      return (
                        <div
                          key={key}
                          className={cn("transition-all", color)}
                          style={{ width: `${pct}%` }}
                        />
                      );
                    })}
                  </div>

                  {/* Legend */}
                  <div className="flex flex-wrap gap-x-4 gap-y-1">
                    {SEGMENTS.map(({ key, label, color }) => (
                      <div
                        key={key}
                        className="text-muted-foreground flex items-center gap-1.5 text-sm"
                      >
                        <div
                          className={cn("h-2.5 w-2.5 rounded-full", color)}
                        />
                        <span>
                          {values[key] ?? 0} {label}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {!hasStudents && (
                <p className="text-muted-foreground mt-2 text-sm">
                  Keine Kinder anwesend
                </p>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}
