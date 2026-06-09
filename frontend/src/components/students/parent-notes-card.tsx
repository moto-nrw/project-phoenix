"use client";

import { useEffect, useState } from "react";
import { MessageCircle } from "lucide-react";
import {
  type StudentParentNote,
  fetchStudentParentNotes,
} from "~/lib/student-parent-notes-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentNotesCard" });

function formatDateTime(iso: string): string {
  try {
    return new Intl.DateTimeFormat("de-DE", {
      day: "2-digit",
      month: "2-digit",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}

/**
 * Read-only card showing the newest messages a guardian left for this
 * child via the parents portal. Self-contained: fetches its own data and
 * renders nothing when there are no notes (or the school has the feature
 * off, which returns an empty list). Staff cannot edit guardian notes.
 */
export function ParentNotesCard({ studentId }: { readonly studentId: string }) {
  const [notes, setNotes] = useState<StudentParentNote[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let active = true;
    fetchStudentParentNotes(studentId)
      .then((n) => {
        if (active) setNotes(n);
      })
      .catch((err) => {
        logger.warn("parent_notes_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: studentId,
        });
      })
      .finally(() => {
        if (active) setLoaded(true);
      });
    return () => {
      active = false;
    };
  }, [studentId]);

  if (!loaded || notes.length === 0) {
    return null;
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center gap-2 sm:gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#F78C10]/10 text-[#F78C10] sm:h-10 sm:w-10">
          <MessageCircle className="h-5 w-5" aria-hidden="true" />
        </div>
        <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
          Elternnachrichten
        </h2>
      </div>
      <ul className="space-y-2">
        {notes.map((note) => (
          <li
            key={note.id}
            className="rounded-xl border border-gray-200 bg-gray-50/70 p-3"
          >
            <p className="text-sm whitespace-pre-wrap text-gray-900">
              {note.body}
            </p>
            <p className="mt-1 text-xs text-gray-500">
              {formatDateTime(note.created_at)}
            </p>
          </li>
        ))}
      </ul>
    </div>
  );
}
