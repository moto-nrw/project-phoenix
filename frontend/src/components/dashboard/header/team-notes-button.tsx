// Header-Hinweis „Neue Notizen vom Team“ (#6854). Erscheint nur, solange
// jemand aus dem Team eine Notiz mit „Speichern und Team informieren“ geteilt
// hat, die diese Person noch nicht gelesen hat. Der rote Zähler verschwindet,
// sobald sie die Notizen des Kindes öffnet.

"use client";

import { useEffect, useState } from "react";
import { NotebookPen } from "lucide-react";
import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Button } from "~/components/ui/button";
import { ListRowLink } from "~/components/ui/list-row-link";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { formatChatTime } from "~/lib/date-helpers";
import { useStudentNoteTeamNotices } from "~/lib/hooks/use-student-note-team-notices";
import type { StudentNoteTeamNotice } from "~/lib/student-notes-api";
import { useTenantAwarePath } from "~/lib/tenant-path";

export const TEAM_NOTES_TITLE = "Neue Notizen vom Team";
const TITLE = TEAM_NOTES_TITLE;

/** The child's notes; opening them clears the marker. */
function teamNoteHref(notice: StudentNoteTeamNotice): string {
  return `/students/${encodeURIComponent(notice.studentId)}?tab=notizen`;
}

function noticeSubtitle(notice: StudentNoteTeamNotice): string {
  const when = formatChatTime(notice.informedAt);
  return notice.authorName ? `Von ${notice.authorName}, ${when}` : when;
}

/**
 * Die geteilten, ungelesenen Notizen als Zeilen: im Popover der Kopfzeile und
 * unter 1024 px im Menü „Mehr“, wo es keine Kopfzeile gibt. Jede Zeile öffnet
 * die Notizen des Kindes; dort wird sie gelesen.
 */
export function TeamNotesList({
  notices,
  onNavigate,
}: {
  readonly notices: readonly StudentNoteTeamNotice[];
  readonly onNavigate?: () => void;
}) {
  const tenantPath = useTenantAwarePath();
  return (
    // Schließt das Menü, wenn eine Zeile den Seitenwechsel startet.
    <ul className="divide-y divide-gray-100" onClickCapture={onNavigate}>
      {notices.map((notice) => (
        <li key={notice.noteId}>
          <ListRowLink
            href={tenantPath(teamNoteHref(notice))}
            title={`${notice.studentFirstName} ${notice.studentLastName}`}
            subtitle={noticeSubtitle(notice)}
            unread
          />
        </li>
      ))}
    </ul>
  );
}

export function TeamNotesButton() {
  const { notices } = useStudentNoteTeamNotices();
  const [isOpen, setIsOpen] = useState(false);
  const count = notices.length;
  // Öffnet die Person eine Notiz, liest der Reiter sie als gelesen; die Liste
  // wird kürzer, und der Hinweis schließt sich.
  useEffect(() => {
    setIsOpen(false);
  }, [count]);

  // Nichts zu lesen, nichts zu zeigen: ein Knopf ohne Inhalt wäre eine
  // Sackgasse.
  if (count === 0) return null;

  return (
    <AnchoredPopover
      open={isOpen}
      onOpenChange={setIsOpen}
      ariaLabel={TITLE}
      preferredWidth={320}
      align="end"
      focusPanelOnOpen
      header={<p className="font-semibold text-gray-900">{TITLE}</p>}
      renderTrigger={({ ref, toggle, panelId }) => (
        <Button
          ref={ref}
          type="button"
          variant="ghost"
          size="icon"
          onClick={toggle}
          aria-label={`${TITLE} (${count})`}
          aria-expanded={isOpen}
          aria-haspopup="dialog"
          aria-controls={isOpen ? panelId : undefined}
          tooltip="Zeigt Notizen, die das Team mit Ihnen geteilt hat."
          className="relative"
        >
          <NotebookPen className="h-5 w-5" aria-hidden="true" />
          <NotificationBadge
            count={count}
            tone="danger"
            size="sm"
            ariaLabel={`${count} ungelesen`}
            className="absolute -top-0.5 -right-0.5"
          />
        </Button>
      )}
    >
      {() => <TeamNotesList notices={notices} />}
    </AnchoredPopover>
  );
}
