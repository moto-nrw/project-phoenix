// Child note card (#3632): API client and type mapping for the Notizen tab
// and the durable hints on the Stammdaten tab.
//
// The backend decides what this reader may see and do: every note arrives
// with can_edit and can_delete already resolved, and notes outside the
// reader's audience never arrive at all. The client renders that answer and
// never computes its own.

import { bffJson } from "./bff-fetch";

/** Note lifetimes. A permanent note is the durable hint shown with the master data. */
export const NOTE_KIND_PERMANENT = "permanent";
export const NOTE_KIND_JOURNAL = "journal";

/** Note audiences, widest first. */
export const NOTE_VISIBILITY_ALL_STAFF = "all_staff";
export const NOTE_VISIBILITY_CARE_TEAM = "care_team";
export const NOTE_VISIBILITY_GROUP_LEADS = "group_leads";

/** A carried-over hint came from the former Betreuernotizen field and has no author. */
export const NOTE_ORIGIN_MASTER_DATA = "master_data";
/** Hinweis der Lehrkraft für einen Tag, geschrieben in moto schule (#4795). */
export const NOTE_ORIGIN_SCHOOL = "school";

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
  /** Für die Schule freigegeben: die Lehrkraft der Klasse liest sie in moto schule (#4797). */
  sharedWithSchool: boolean;
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
  /** Für die Schule freigeben (#4797); fehlt der Wert, bleibt die Freigabe aus. */
  sharedWithSchool?: boolean;
}

/** An activity (Angebot) of the child that a new note may refer to. */
export interface StudentNoteActivity {
  id: string;
  name: string;
}

interface BackendNoteSubjects {
  activities?: { id: string; name: string }[];
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
  shared_with_school?: boolean;
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
    sharedWithSchool: data.shared_with_school ?? false,
    canEdit: data.can_edit,
    canDelete: data.can_delete,
    edited: data.edited ?? false,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

/** A note a colleague shared with the team that this reader has not read (#6854). */
export interface StudentNoteTeamNotice {
  noteId: string;
  studentId: string;
  studentFirstName: string;
  studentLastName: string;
  authorName: string;
  informedAt: string;
}

export type StudentNoteTeamNoticeRevision = Pick<
  StudentNoteTeamNotice,
  "noteId" | "informedAt"
>;

interface BackendTeamNotice {
  note_id: string;
  student_id: string;
  student_first_name: string;
  student_last_name: string;
  author_name?: string;
  informed_at: string;
}

function mapTeamNotice(data: BackendTeamNotice): StudentNoteTeamNotice {
  return {
    noteId: data.note_id,
    studentId: data.student_id,
    studentFirstName: data.student_first_name,
    studentLastName: data.student_last_name,
    authorName: data.author_name ?? "",
    informedAt: data.informed_at,
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
    shared_with_school: draft.sharedWithSchool ?? false,
  };
}

function notesPath(studentId: string, noteId?: string): string {
  const base = `/api/students/${encodeURIComponent(studentId)}/notes`;
  return noteId ? `${base}/${encodeURIComponent(noteId)}` : base;
}

class StudentNotesService {
  /** kind narrows the read to one lifetime; omit it for the whole card. */
  async list(studentId: string, kind?: string): Promise<StudentNote[]> {
    const query = kind ? `?kind=${encodeURIComponent(kind)}` : "";
    const notes = await bffJson<BackendStudentNote[] | null>(
      `${notesPath(studentId)}${query}`,
    );
    return (notes ?? []).map(mapNote);
  }

  /**
   * The child's activities today. The create route checks a reference
   * against the same read, so the form never offers one it would refuse.
   */
  async listActivities(studentId: string): Promise<StudentNoteActivity[]> {
    const subjects = await bffJson<BackendNoteSubjects | null>(
      `/api/students/${encodeURIComponent(studentId)}/note-subjects`,
    );
    return (subjects?.activities ?? []).map((activity) => ({
      id: activity.id,
      name: activity.name,
    }));
  }

  /** Returns the note as stored, so a follow-up action can name it. */
  async create(
    studentId: string,
    draft: StudentNoteDraft,
  ): Promise<StudentNote> {
    const note = await bffJson<BackendStudentNote>(notesPath(studentId), {
      method: "POST",
      body: toPayload(draft),
    });
    return mapNote(note);
  }

  async update(
    studentId: string,
    noteId: string,
    draft: StudentNoteDraft,
  ): Promise<StudentNote> {
    const note = await bffJson<BackendStudentNote>(
      notesPath(studentId, noteId),
      { method: "PUT", body: toPayload(draft) },
    );
    return mapNote(note);
  }

  /**
   * „Speichern und Team informieren“ (#6854): tells the colleagues who may
   * read the child's notes about a saved note. Returns how many were told;
   * zero means nobody else in the team may read the notes.
   */
  async informTeam(studentId: string, noteId: string): Promise<number> {
    const result = await bffJson<{ informed: number }>(
      `${notesPath(studentId, noteId)}/team-notices`,
      { method: "POST", body: {} },
    );
    return result.informed;
  }

  /** The reader's notes shared by the team and not yet read, newest first. */
  async listTeamNotices(): Promise<StudentNoteTeamNotice[]> {
    const notices = await bffJson<BackendTeamNotice[] | null>(
      "/api/students/team-notices",
    );
    return (notices ?? []).map(mapTeamNotice);
  }

  /**
   * Marks only the announcement revisions loaded by the reader as read;
   * answers with the notes still unread.
   */
  async markTeamNoticesRead(
    studentId: string,
    observed: readonly StudentNoteTeamNoticeRevision[],
  ): Promise<StudentNoteTeamNotice[]> {
    const notices = await bffJson<BackendTeamNotice[] | null>(
      `/api/students/${encodeURIComponent(studentId)}/team-notices/read`,
      {
        method: "POST",
        body: {
          notices: observed.map((notice) => ({
            note_id: notice.noteId,
            informed_at: notice.informedAt,
          })),
        },
      },
    );
    return (notices ?? []).map(mapTeamNotice);
  }

  async remove(studentId: string, noteId: string): Promise<void> {
    await bffJson<unknown>(notesPath(studentId, noteId), { method: "DELETE" });
  }
}

export const studentNotesService = new StudentNotesService();
