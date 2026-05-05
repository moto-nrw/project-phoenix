// components/rooms/students-in-room-section.tsx
//
// Live "Kinder im Raum" view inside the room-detail slide-over (#1323).
//
// Uses the same `/api/students?room_id=` data path as Kindersuche, but
// renders a compact name + class + group row per child (CompactStudentCard)
// because the side panel doesn't have room — and doesn't need — the full
// StudentCard. Every row shown is a real, currently checked-in student;
// the total count is the server's authoritative pagination count.
//
// Live updates: SSE in `use-global-sse` invalidates `room-students-{roomId}`
// on student_checkin / student_checkout / activity_start / activity_end /
// dashboard_counts_changed events.

"use client";

import { useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSWRAuth } from "~/lib/swr";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import { CompactStudentCard } from "~/components/students/compact-student-card";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsInRoomSection" });

interface StudentsInRoomSectionProps {
  readonly roomId: string;
  readonly roomName: string;
}

export function StudentsInRoomSection({
  roomId,
  roomName,
}: StudentsInRoomSectionProps) {
  const router = useTenantRouter();
  const sectionSearchParams = useSearchParams();
  // Drilling into a child must return to the same /rooms grid state the
  // user came from. Preserve the full query string (room=… AND any
  // filter params like ?search=…&building=…&status=…) so the user lands
  // back on the same narrowed view with the slide-over reopened.
  const fromReferrer = (() => {
    const qs = sectionSearchParams?.toString() ?? "";
    return qs ? `/rooms?${qs}` : `/rooms?room=${roomId}`;
  })();

  // pageSize must comfortably exceed any realistic room occupancy (combined
  // groups in Sporthalle/Aula/Mensa cap well below 100 in normal usage) so
  // every present child shows up. Backend default is only 50, which would
  // silently truncate larger rooms — see PR #1374 review.
  const { data, error, isLoading } = useSWRAuth<{
    students: Student[];
    pagination?: { total_records: number };
  }>(`room-students-${roomId}`, async () =>
    studentService.getStudents({
      roomId,
      pageSize: 200,
    }),
  );

  if (error) {
    logger.warn("students_in_room_load_failed", {
      room_id: roomId,
      error: error instanceof Error ? error.message : String(error),
    });
  }

  const students = data?.students ?? [];
  // Prefer the server's authoritative count so the badge stays honest even
  // if pagination ever truncates the response. Fall back to length only
  // when pagination metadata is missing.
  const totalCount = data?.pagination?.total_records ?? students.length;
  // If the response is truncated (room has more occupants than pageSize),
  // surface the overflow + offer the Kindersuche escape hatch (#1374).
  // Without this notice, staff see N cards while the badge says >N and
  // have no affordance to open the missing children.
  const isTruncated = totalCount > students.length;
  const hiddenCount = Math.max(0, totalCount - students.length);

  const openInSearch = () => {
    const qs = new URLSearchParams({
      room_id: roomId,
      room_name: roomName,
    }).toString();
    router.push(`/students/search?${qs}`);
  };

  return (
    // Quiet section header to match the other slide-over blocks
    // (Rauminformationen / Belegungshistorie). Section element +
    // aria-label preserve the "info-card" landmark contract the tests
    // / a11y tooling expect from the previous InfoCard wrapper.
    <section
      aria-label="Kinder im Raum"
      className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6"
    >
      <h2 className="mb-3 text-xs font-semibold tracking-wider text-gray-500 uppercase">
        Kinder im Raum
      </h2>
      {/* items-end so the Kindersuche button bottom-aligns with the
          count text bottom. The shared `<Button>` component has fixed
          py-3, which would make the row too tall and push the count
          text down — so this row uses an inline outline-styled button
          with py-1, matching the count text's line height. The button
          stays the same visual style as the elsewhere-used Button
          outline variant, just sized to fit a one-line row.
          Review feedback (#1323). */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <p className="text-sm text-gray-600">
          <span className="font-medium text-gray-900">{totalCount}</span>{" "}
          {totalCount === 1 ? "Kind" : "Kinder"} aktuell anwesend
        </p>
        {totalCount > 0 && (
          // Tinted brand-blue ghost button. Uses LOCATION_COLORS.OTHER_ROOM
          // (#5080D8) — same hue the IconDetailRow icons in
          // Rauminformationen above are tinted with, so the slide-over
          // has one accent color across all its sections instead of the
          // gray-outline button visually getting lost. CLAUDE.md §0 —
          // brand colors via arbitrary value syntax, never generic
          // tailwind blues.
          <button
            type="button"
            onClick={openInSearch}
            className="inline-flex items-center rounded-lg bg-[#5080D8]/10 px-3 py-1 text-sm font-medium text-[#5080D8] transition-colors hover:bg-[#5080D8]/20 focus:ring-2 focus:ring-[#5080D8]/40 focus:outline-none"
          >
            In Kindersuche öffnen
          </button>
        )}
      </div>

      {isTruncated && (
        <div
          role="status"
          className="mt-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900"
        >
          Es werden {students.length} von {totalCount} Kindern angezeigt.{" "}
          {hiddenCount} weitere {hiddenCount === 1 ? "Kind ist" : "Kinder sind"}{" "}
          aktuell nicht in dieser Übersicht — bitte in der Kindersuche öffnen,
          um alle zu sehen.
        </div>
      )}

      {/* Breathing room between the count + Kindersuche button row and
          the first student card. Without it the button bottom edge sits
          flush with the first card border. Review feedback (#1323). */}
      <div className="mt-4">
        <StudentsInRoomBody
          fromReferrer={fromReferrer}
          loading={isLoading}
          hasError={!!error}
          students={students}
          router={router}
        />
      </div>
    </section>
  );
}

interface StudentsInRoomBodyProps {
  readonly fromReferrer: string;
  readonly loading: boolean;
  readonly hasError: boolean;
  readonly students: readonly Student[];
  readonly router: ReturnType<typeof useTenantRouter>;
}

function StudentsInRoomBody({
  fromReferrer,
  loading,
  hasError,
  students,
  router,
}: StudentsInRoomBodyProps) {
  if (hasError) {
    return (
      <Alert
        type="error"
        message="Die Liste der Kinder konnte nicht geladen werden."
      />
    );
  }

  if (loading && students.length === 0) {
    return <Loading message="Kinderliste wird geladen..." fullPage={false} />;
  }

  if (students.length === 0) {
    return (
      <div className="py-8 text-center text-gray-500">
        Aktuell keine Kinder im Raum.
      </div>
    );
  }

  return (
    // Single column inside the slide-over: the panel is ~512px, so a
    // multi-column grid would either truncate names or shrink the rows
    // below useful tap-target size on touch.
    <div className="flex flex-col gap-2">
      {students.map((student) => (
        <CompactStudentCard
          key={student.id}
          studentId={student.id}
          firstName={student.first_name}
          lastName={student.second_name}
          schoolClass={student.school_class}
          groupName={student.group_name ?? undefined}
          onClick={() =>
            router.push(
              `/students/${student.id}?from=${encodeURIComponent(fromReferrer)}`,
            )
          }
        />
      ))}
    </div>
  );
}
