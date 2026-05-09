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

interface CompactStudentCardProps {
  readonly studentId: string | number;
  readonly firstName?: string;
  readonly lastName?: string;
  readonly schoolClass?: string;
  readonly groupName?: string;
  readonly onClick?: () => void;
}

export function CompactStudentCard({
  studentId,
  firstName,
  lastName,
  schoolClass,
  groupName,
  onClick,
}: CompactStudentCardProps) {
  // No "Gruppe X" prefix — group names are already self-evidently
  // group names ("Bärengruppe", "Sternengruppe", …), so prefixing with
  // "Gruppe" reads as a stutter ("Gruppe Bärengruppe").
  const meta = [schoolClass, groupName].filter(Boolean).join(" · ");

  return (
    <button
      key={studentId}
      type="button"
      onClick={onClick}
      aria-label={`${firstName} ${lastName} – Profil öffnen`}
      data-testid={`compact-student-card-${studentId}`}
      className="group w-full rounded-xl border border-gray-200 bg-white px-4 py-3 text-left transition-colors duration-150 hover:border-gray-300 hover:bg-gray-50 focus:ring-2 focus:ring-[#5080D8]/40 focus:outline-none"
    >
      <p className="truncate text-base font-semibold text-gray-900">
        {firstName} {lastName}
      </p>
      {meta && <p className="mt-0.5 truncate text-sm text-gray-500">{meta}</p>}
    </button>
  );
}
