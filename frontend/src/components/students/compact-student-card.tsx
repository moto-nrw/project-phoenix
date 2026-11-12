// components/students/compact-student-card.tsx
//
// Lightweight, single-line student row for the room-detail slide-over
// (#1323 review). The full StudentCard is too heavy for a side panel,
// staff already know the kids are present (that's the whole point of
// "Kinder im Raum"), so the location badge and arrival/pickup rows are
// noise here. Just the identifiers staff need to recognise the child:
// name, class, group.
//
// Used by the room-detail slide-over and its sibling sections. Kindersuche /
// database list views keep the rich `StudentCard` because the badge + times
// carry real information there.

"use client";

import { ParentAttentionBadge } from "~/components/students/parent-attention-badge";
import { Avatar } from "~/components/ui/avatar";
import { useParentAttentionEnabled } from "~/lib/hooks/use-parent-attention";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";

interface CompactStudentCardProps {
  readonly studentId: string | number;
  readonly firstName?: string;
  readonly lastName?: string;
  readonly schoolClass?: string;
  readonly groupName?: string;
  /**
   * Photo URL, gated server-side by operations.student_photos_enabled +
   * parental consent. When falsy the Avatar falls back to brand initials,
   * so callers can pass `student.photo_url` unconditionally.
   */
  readonly photoUrl?: string | null;
  readonly onClick?: () => void;
  readonly chrome?: "card" | "plain";
  readonly className?: string;
}

export function CompactStudentCard({
  studentId,
  firstName,
  lastName,
  schoolClass,
  groupName,
  photoUrl,
  onClick,
  chrome = "card",
  className = "",
}: CompactStudentCardProps) {
  // Per-tenant feature flag. When off, the avatar slot is suppressed so
  // opt-out schools see the original pre-feature shape (name + meta only).
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  const parentAttentionEnabled = useParentAttentionEnabled();
  // No "Gruppe X" prefix, group names are already self-evidently
  // group names ("Bärengruppe", "Sternengruppe", …), so prefixing with
  // "Gruppe" reads as a stutter ("Gruppe Bärengruppe").
  const meta = [schoolClass, groupName].filter(Boolean).join(" · ");
  const fullName = `${firstName ?? ""} ${lastName ?? ""}`.trim() || "?";

  const chromeClass =
    chrome === "card"
      ? "moto-content-surface border px-4 py-3"
      : "bg-transparent px-0 py-0 hover:bg-transparent";
  const content = (
    <>
      {photosEnabled ? (
        <Avatar imageUrl={photoUrl ?? null} name={fullName} size="sm" />
      ) : null}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-gray-900">
          {firstName} {lastName}
        </p>
        {meta && (
          <p className="mt-0.5 truncate text-xs text-gray-500">{meta}</p>
        )}
      </div>
    </>
  );

  let card: React.ReactElement;
  if (!onClick) {
    card = (
      <div
        data-testid={`compact-student-card-${studentId}`}
        className={`flex w-full items-center gap-3 rounded-xl text-left ${chromeClass} ${className}`}
      >
        {content}
      </div>
    );
  } else {
    card = (
      <button
        key={studentId}
        type="button"
        onClick={onClick}
        aria-label={`${firstName} ${lastName} - Profil öffnen`}
        data-testid={`compact-student-card-${studentId}`}
        className={`group moto-hover-elevated flex w-full items-center gap-3 rounded-xl text-left transition-colors duration-150 focus:outline-none focus-visible:outline-2 focus-visible:outline-gray-300 ${chromeClass} ${className}`}
      >
        {content}
      </button>
    );
  }

  if (!parentAttentionEnabled) return card;
  // Neue Elternnachrichten (#6486), wie an der großen Kinderkachel.
  return (
    <div className="relative">
      {card}
      <ParentAttentionBadge
        studentId={String(studentId)}
        studentName={fullName}
        className="absolute -top-3 -right-3 z-10"
      />
    </div>
  );
}
