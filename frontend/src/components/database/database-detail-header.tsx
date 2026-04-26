"use client";

import type { ReactNode } from "react";

interface DatabaseDetailHeaderProps {
  avatar: string;
  title: string;
  subtitle: string;
  actions?: ReactNode;
}

export function DatabaseDetailHeader({
  avatar,
  title,
  subtitle,
  actions,
}: DatabaseDetailHeaderProps) {
  return (
    <div className="flex items-center gap-4 px-6 py-5">
      <div
        aria-hidden
        className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-[#DCE6F8] text-base font-semibold text-[#5080D8]"
      >
        {avatar}
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-lg font-bold text-gray-900">{title}</div>
        <div className="mt-0.5 truncate text-sm text-gray-500">{subtitle}</div>
      </div>
      {actions ? (
        <div className="flex items-center gap-2">{actions}</div>
      ) : null}
    </div>
  );
}
