"use client";

import Link from "next/link";
import { ChevronRight } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "~/components/ui/card";
import { Skeleton } from "~/components/ui/skeleton";
import type { LucideIcon } from "lucide-react";

interface DashboardListCardProps {
  readonly title: string;
  readonly icon: LucideIcon;
  readonly href?: string;
  readonly children: React.ReactNode;
  readonly loading?: boolean;
  readonly skeletonCount?: number;
  readonly isEmpty?: boolean;
}

export function DashboardListCard({
  title,
  icon: Icon,
  href,
  children,
  loading = false,
  skeletonCount = 3,
  isEmpty = false,
}: DashboardListCardProps) {
  // Hide entirely when empty and not loading
  if (isEmpty && !loading) return null;

  const content = (
    <Card className="h-full transition-shadow group-hover:shadow-md">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Icon className="text-muted-foreground h-4 w-4" />
            <CardTitle className="text-base">{title}</CardTitle>
          </div>
          {href && (
            <ChevronRight className="text-muted-foreground h-4 w-4 transition-transform group-hover:translate-x-0.5" />
          )}
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: skeletonCount }, (_, i) => (
              <Skeleton key={i} className="h-12 w-full rounded-lg" />
            ))}
          </div>
        ) : (
          children
        )}
      </CardContent>
    </Card>
  );

  if (href) {
    return (
      <Link
        href={href}
        className="group block rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
      >
        {content}
      </Link>
    );
  }

  return <div className="group">{content}</div>;
}
