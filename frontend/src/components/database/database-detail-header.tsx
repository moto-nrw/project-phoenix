"use client";

import { AlertTriangle } from "lucide-react";
import type { ReactNode } from "react";
import { Avatar } from "~/components/ui/avatar";

/**
 * `avatar` accepts either:
 *  - a plain string of pre-computed initials (legacy callers: roles, groups,
 *    permissions where the initials don't derive cleanly from a name field), or
 *  - an object with `{ name, imageUrl? }` so callers with a real entity name
 *    can pass through the shared <Avatar> (image with initials fallback).
 *
 * Existing string callers stay pixel-identical via the legacy chip below; the
 * object form is used for entities that may have a photo (students today).
 */
type AvatarProp = string | { name: string; imageUrl?: string | null };

interface DatabaseDetailHeaderProps {
  avatar?: AvatarProp;
  icon?: ReactNode;
  title: string;
  subtitle: string;
  /** Optional warning chip rendered between subtitle and actions. */
  warning?: string | null;
  actions?: ReactNode;
}

export function DatabaseDetailHeader({
  avatar,
  icon,
  title,
  subtitle,
  warning,
  actions,
}: DatabaseDetailHeaderProps) {
  return (
    <div className="flex items-center gap-4 px-6 py-5">
      {icon ? (
        <div className="flex h-12 w-12 shrink-0 items-center justify-center">
          {icon}
        </div>
      ) : typeof avatar === "string" ? (
        // Legacy chip — kept verbatim so roles/groups/permissions render
        // identically to before. Light-blue brand chip with pre-computed
        // initials. Do not migrate these callers without designer review.
        <div
          aria-hidden
          className="text-moto-blue flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-[#DCE6F8] text-base font-semibold"
        >
          {avatar}
        </div>
      ) : avatar ? (
        <Avatar imageUrl={avatar.imageUrl} name={avatar.name} size="md" />
      ) : null}
      <div className="min-w-0 flex-1">
        <div className="truncate text-lg font-bold text-gray-900">{title}</div>
        <div className="mt-0.5 truncate text-sm text-gray-500">{subtitle}</div>
      </div>
      {warning ? (
        <div className="text-moto-orange flex items-center gap-1.5 rounded-full bg-[#FFE8D0] px-3 py-1 text-xs font-semibold">
          <AlertTriangle className="h-3 w-3" aria-hidden />
          {warning}
        </div>
      ) : null}
      {actions ? (
        <div className="flex items-center gap-2">{actions}</div>
      ) : null}
    </div>
  );
}
