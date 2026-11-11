"use client";

import type { ReactNode } from "react";
import { Check, X } from "lucide-react";
import {
  BuildingsIcon,
  BusIcon,
  FileTextIcon,
  HeartIcon,
  NotePencilIcon,
  UserIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { InsetPanel } from "~/components/ui/inset-panel";

// Data-field primitives live in data-field.tsx, so a kit card can
// use them without loading the icon constants below (#5581).
export {
  DataField,
  DataFieldSkeleton,
  DataGrid,
} from "~/components/ui/data-field";

// =============================================================================
// InfoSection - Shared component for information section containers
// =============================================================================

interface InfoSectionProps {
  readonly title: string;
  /** Optional leading glyph; a titled group without one renders text only. */
  readonly icon?: ReactNode;
  readonly children: ReactNode;
}

/**
 * Container component for information sections in detail modals.
 * Neutral surface (no accent-tinted backgrounds); the icon supplies its own
 * color (typically a MotoDuotoneIcon/MotoConceptIcon).
 */
export function InfoSection({ title, icon, children }: InfoSectionProps) {
  return (
    <InsetPanel framed padding="md">
      <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
        {icon ? (
          <span className="h-3.5 w-3.5 md:h-4 md:w-4">{icon}</span>
        ) : null}
        {title}
      </h3>
      {children}
    </InsetPanel>
  );
}

// =============================================================================
// InfoText - Shared component for text-only info sections
// =============================================================================

interface InfoTextProps {
  readonly children: ReactNode;
}

/**
 * Text content for info sections that don't use a data grid.
 */
export function InfoText({ children }: InfoTextProps) {
  return (
    <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
      {children}
    </p>
  );
}

// =============================================================================
// Common Icons - Shared SVG icons for detail modals
// =============================================================================

export const DetailIcons = {
  // Domain-flavored glyphs: self-colored via MotoDuotoneIcon.
  person: <MotoDuotoneIcon icon={UserIcon} tone="greenVivid" size="100%" />,
  group: <MotoDuotoneIcon icon={UsersThreeIcon} tone="greenDeep" size="100%" />,
  heart: <MotoDuotoneIcon icon={HeartIcon} tone="coral" size="100%" />,
  notes: <MotoDuotoneIcon icon={NotePencilIcon} tone="teal" size="100%" />,
  document: <MotoDuotoneIcon icon={FileTextIcon} tone="neutral" size="100%" />,
  building: <MotoDuotoneIcon icon={BuildingsIcon} tone="petrol" size="100%" />,
  bus: <MotoDuotoneIcon icon={BusIcon} tone="cyan" size="100%" />,
  // Generic UI feedback: plain Lucide icons inheriting currentColor, so
  // callers that tint them via a wrapping text-color class keep working.
  check: <Check className="h-full w-full" strokeWidth={2} />,
  x: <X className="h-full w-full" strokeWidth={2} />,
} as const;
