"use client";

import type { ReactNode } from "react";

interface DatabaseEmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
}

export function DatabaseEmptyState({
  icon,
  title,
  description,
}: DatabaseEmptyStateProps) {
  return (
    <div className="flex min-h-[300px] items-center justify-center">
      <div className="text-center">
        {icon}
        <h3 className="mt-4 text-lg font-medium text-gray-900">{title}</h3>
        <p className="mt-2 text-sm text-gray-600">{description}</p>
      </div>
    </div>
  );
}
