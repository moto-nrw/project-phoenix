// components/students/compact-student-card.tsx
//
// Lightweight, single-line student row for the room-detail slide-over
// (#1323 review). The full StudentCard is too heavy for a side panel —
// staff already know the kids are present (that's the whole point of
// "Kinder im Raum"), so the location badge and arrival/pickup rows are
// noise here. Just the identifiers staff need to recognise the child:
// name, class, group.
//
// Used by `students-in-room-section.tsx` only. Kindersuche / database
// list views keep the rich `StudentCard` because the badge + times
// carry real information there.

"use client";

import { Avatar } from "~/components/ui/avatar";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";

interface CompactStudentCardProps {
  readonly studentId: string | number;
  readonly firstName?: string;
  readonly lastName?: string;
  readonly schoolClass?: string;
  readonly groupName?: string;
  /**
   * Photo URL — gated server-side by operations.student_photos_enabled +
   * parental consent. When falsy the Avatar falls back to brand initials,
   * so callers can pass `student.photo_url` unconditionally.
   */
  readonly photoUrl?: string | null;
  readonly onClick?: () => void;
}

export function CompactStudentCard({
  studentId,
  firstName,
  lastName,
  schoolClass,
  groupName,
  photoUrl,
  onClick,
}: CompactStudentCardProps) {
  // Per-tenant feature flag. When off, the avatar slot is suppressed so
  // opt-out schools see the original pre-feature shape (name + meta only).
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  // No "Gruppe X" prefix — group names are already self-evidently
  // group names ("Bärengruppe", "Sternengruppe", …), so prefixing with
  // "Gruppe" reads as a stutter ("Gruppe Bärengruppe").
  const meta = [schoolClass, groupName].filter(Boolean).join(" · ");
  const fullName = `${firstName ?? ""} ${lastName ?? ""}`.trim() || "?";

  return (
    <button
      key={studentId}
      type="button"
      onClick={onClick}
      aria-label={`${firstName} ${lastName} – Profil öffnen`}
      data-testid={`compact-student-card-${studentId}`}
      className="group flex w-full items-center gap-3 rounded-xl border border-gray-200 bg-white px-4 py-3 text-left transition-colors duration-150 hover:border-gray-300 hover:bg-gray-50 focus:ring-2 focus:ring-[#5080D8]/40 focus:outline-none"
    >
      {photosEnabled ? (
        <Avatar imageUrl={photoUrl ?? null} name={fullName} size="sm" />
      ) : null}
      <div className="min-w-0 flex-1">
        <p className="truncate text-base font-semibold text-gray-900">
          {firstName} {lastName}
        </p>
        {meta && (
          <p className="mt-0.5 truncate text-sm text-gray-500">{meta}</p>
        )}
      </div>
    </button>
  );
}
