// Child note card (#3632): API client and type mapping for the Notizen tab
// and the durable hints on the Stammdaten tab.
//
// The backend decides what this reader may see and do: every note arrives
// with can_edit and can_delete already resolved, and notes outside the
// reader's audience never arrive at all. The client renders that answer and
// never computes its own.

import { sessionFetch } from "./session-cache";

/** Note lifetimes. A permanent note is the durable hint shown with the master data. */
export const NOTE_KIND_PERMANENT = "permanent";
export const NOTE_KIND_JOURNAL = "journal";

/** Note audiences, widest first. */
export const NOTE_VISIBILITY_ALL_STAFF = "all_staff";
export const NOTE_VISIBILITY_CARE_TEAM = "care_team";
export const NOTE_VISIBILITY_GROUP_LEADS = "group_leads";

/** A carried-over hint came from the former Betreuernotizen field and has no author. */
export const NOTE_ORIGIN_MASTER_DATA = "master_data";

export interface StudentNote {
  id: string;
  studentId: string;
  kind: string;
  visibility: string;
  category: string;
  body: string;
  origin: string;
  authorName: string;
  /** Calendar day the entry describes, "YYYY-MM-DD"; empty for a durable hint. */
  subjectDate: string;
  activityGroupId: string;
  educationGroupId: string;
  canEdit: boolean;
  canDelete: boolean;
  /** The wording changed after the note was written. */
  edited: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface StudentNoteDraft {
  kind: string;
  visibility: string;
  category?: string;
  body: string;
  subjectDate?: string;
  activityGroupId?: string;
  educationGroupId?: string;
}

interface BackendStudentNote {
  id: string;
  student_id: string;
  kind: string;
  visibility: string;
  category?: string;
  body: string;
  origin: string;
  author_name?: string;
  subject_date?: string;
  activity_group_id?: string;
  education_group_id?: string;
  can_edit: boolean;
  can_delete: boolean;
  edited?: boolean;
  created_at: string;
  updated_at: string;
}

function mapNote(data: BackendStudentNote): StudentNote {
  return {
    id: data.id,
    studentId: data.student_id,
    kind: data.kind,
    visibility: data.visibility,
    category: data.category ?? "",
    body: data.body,
    origin: data.origin,
    authorName: data.author_name ?? "",
    subjectDate: data.subject_date ?? "",
    activityGroupId: data.activity_group_id ?? "",
    educationGroupId: data.education_group_id ?? "",
    canEdit: data.can_edit,
    canDelete: data.can_delete,
    edited: data.edited ?? false,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

function toPayload(draft: StudentNoteDraft): Record<string, unknown> {
  return {
    kind: draft.kind,
    visibility: draft.visibility,
    category: draft.category ?? "",
    body: draft.body,
    subject_date: draft.subjectDate ?? null,
    activity_group_id: draft.activityGroupId ?? null,
    education_group_id: draft.educationGroupId ?? null,
  };
}

function throwNoteError(response: Response, fallback: string): never {
  let message = fallback;
  if (response.status === 403) {
    message = "Dafür fehlt Ihnen die Berechtigung.";
  }
  throw new Error(message);
}

class StudentNotesService {
  /** kind narrows the read to one lifetime; omit it for the whole card. */
  async list(studentId: string, kind?: string): Promise<StudentNote[]> {
    const query = kind ? `?kind=${encodeURIComponent(kind)}` : "";
    const response = await sessionFetch(
      `/api/students/${studentId}/notes${query}`,
    );
    if (!response.ok) {
      throwNoteError(response, "Notizen konnten nicht geladen werden.");
    }
    const json = (await response.json()) as { data: BackendStudentNote[] };
    return (json.data ?? []).map(mapNote);
  }

  async create(studentId: string, draft: StudentNoteDraft): Promise<void> {
    const response = await sessionFetch(`/api/students/${studentId}/notes`, {
      method: "POST",
      body: JSON.stringify(toPayload(draft)),
    });
    if (!response.ok) {
      throwNoteError(response, "Notiz konnte nicht gespeichert werden.");
    }
  }

  async update(
    studentId: string,
    noteId: string,
    draft: StudentNoteDraft,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/students/${studentId}/notes/${noteId}`,
      { method: "PUT", body: JSON.stringify(toPayload(draft)) },
    );
    if (!response.ok) {
      throwNoteError(response, "Notiz konnte nicht geändert werden.");
    }
  }

  async remove(studentId: string, noteId: string): Promise<void> {
    const response = await sessionFetch(
      `/api/students/${studentId}/notes/${noteId}`,
      { method: "DELETE" },
    );
    if (!response.ok) {
      throwNoteError(response, "Notiz konnte nicht entfernt werden.");
    }
  }
}

export const studentNotesService = new StudentNotesService();
