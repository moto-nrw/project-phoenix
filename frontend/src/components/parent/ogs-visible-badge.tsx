"use client";

import { Eye } from "lucide-react";
import { useTranslations } from "next-intl";

import { cn } from "~/lib/utils";

export function OgsVisibleBadge({
  className,
}: Readonly<{ className?: string }>) {
  const t = useTranslations("parentVisibility");

  return (
    <span
      className={cn(
        "inline-flex min-h-8 items-center gap-1.5 rounded-full bg-gray-100 px-3 text-xs font-medium text-gray-600",
        className,
      )}
      aria-label={t("ogsDescription")}
    >
      <Eye className="size-4 shrink-0" aria-hidden="true" />
      {t("ogs")}
    </span>
  );
}
