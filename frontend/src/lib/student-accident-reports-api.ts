// Accident reports (Unfallanzeige, #4771): the recorded accident of a child.
// A report is a child document of the category „Unfallanzeige“; the list
// arrives with the documents (`student-documents-api`). This client writes
// reports and names the PDF. The backend decides every permission
// (student_accidents:read / :manage) and refuses changes to a signed report.

import { ApiError, apiErrorFromResponse, transportFetch } from "./api-error";
import { hasSessionAccess } from "./session-access";
import { getCachedSession, sessionFetch } from "./session-cache";
import { signatureRequestBody, type SignatureValue } from "./signature";

type AccidentReportStatus = "open" | "signed";

/** What the team records, in the fields of the official form. */
export interface AccidentReportFields {
  /** Instant of the accident, ISO 8601. */
  occurredAt: string;
  location: string;
  description: string;
  injuredBodyParts: string;
  injuryKind: string;
  witnesses: string;
  firstAid: string;
  doctorVisited: boolean;
  doctor: string;
  parentsNotified: string;
  /** Instant the parents were told, ISO 8601, or null. */
  parentsNotifiedAt: string | null;
  supervisor: string;
}

export interface AccidentReportDetails extends AccidentReportFields {
  status: AccidentReportStatus;
  /** 1 for the first report, one more per correction; 0 when unknown. */
  version: number;
  supersedesId: string | null;
  supersededById: string | null;
  signedAt: string | null;
  signedDigitally: boolean;
}

export interface BackendAccidentReport {
  status: AccidentReportStatus;
  version?: number;
  supersedes_id?: string;
  superseded_by_id?: string;
  signed_at?: string;
  signed_digitally: boolean;
  occurred_at: string;
  location: string;
  description: string;
  injured_body_parts: string;
  injury_kind: string;
  witnesses: string;
  first_aid: string;
  doctor_visited: boolean;
  doctor: string;
  parents_notified: string;
  parents_notified_at?: string;
  supervisor: string;
}

export function mapAccidentReport(
  data: BackendAccidentReport,
): AccidentReportDetails {
  return {
    status: data.status,
    version: data.version ?? 0,
    supersedesId: data.supersedes_id ?? null,
    supersededById: data.superseded_by_id ?? null,
    signedAt: data.signed_at ?? null,
    signedDigitally: data.signed_digitally,
    occurredAt: data.occurred_at,
    location: data.location,
    description: data.description,
    injuredBodyParts: data.injured_body_parts,
    injuryKind: data.injury_kind,
    witnesses: data.witnesses,
    firstAid: data.first_aid,
    doctorVisited: data.doctor_visited,
    doctor: data.doctor,
    parentsNotified: data.parents_notified,
    parentsNotifiedAt: data.parents_notified_at ?? null,
    supervisor: data.supervisor,
  };
}

function requestBody(fields: AccidentReportFields) {
  return {
    occurred_at: fields.occurredAt,
    location: fields.location,
    description: fields.description,
    injured_body_parts: fields.injuredBodyParts,
    injury_kind: fields.injuryKind,
    witnesses: fields.witnesses,
    first_aid: fields.firstAid,
    doctor_visited: fields.doctorVisited,
    doctor: fields.doctorVisited ? fields.doctor : "",
    parents_notified: fields.parentsNotified,
    parents_notified_at: fields.parentsNotified
      ? fields.parentsNotifiedAt
      : null,
    supervisor: fields.supervisor,
  };
}

async function reportId(response: Response, fallback: string): Promise<string> {
  if (!response.ok) {
    throw await apiErrorFromResponse(response, fallback);
  }
  const json = (await response.json()) as { data: { id: number } };
  return json.data.id.toString();
}

/** The page of one accident report in the tenant portal. */
export function accidentReportPath(studentId: string, reportId: string) {
  return `/students/${studentId}/unfallanzeigen/${reportId}`;
}

class StudentAccidentReportsService {
  /** Records a new open report and returns its id. */
  async create(
    studentId: string,
    fields: AccidentReportFields,
  ): Promise<string> {
    const response = await sessionFetch(
      `/api/students/${studentId}/accident-reports`,
      { method: "POST", body: JSON.stringify(requestBody(fields)) },
    );
    return reportId(response, "Unfallanzeige konnte nicht gespeichert werden.");
  }

  /** Changes an open report. */
  async update(
    studentId: string,
    reportIdValue: string,
    fields: AccidentReportFields,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/students/${studentId}/accident-reports/${reportIdValue}`,
      { method: "PUT", body: JSON.stringify(requestBody(fields)) },
    );
    await reportId(response, "Unfallanzeige konnte nicht gespeichert werden.");
  }

  /** Records a new version of a signed report and returns its id. */
  async correct(
    studentId: string,
    reportIdValue: string,
    fields: AccidentReportFields,
  ): Promise<string> {
    const response = await sessionFetch(
      `/api/students/${studentId}/accident-reports/${reportIdValue}/corrections`,
      { method: "POST", body: JSON.stringify(requestBody(fields)) },
    );
    return reportId(response, "Korrektur konnte nicht gespeichert werden.");
  }

  async sign(
    studentId: string,
    reportIdValue: string,
    signature: SignatureValue,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/students/${studentId}/accident-reports/${reportIdValue}/signature`,
      { method: "POST", body: JSON.stringify(signatureRequestBody(signature)) },
    );
    await reportId(response, "Unterschrift konnte nicht gespeichert werden.");
  }

  async uploadPaperCopy(
    studentId: string,
    reportIdValue: string,
    file: File,
  ): Promise<void> {
    if (!hasSessionAccess(await getCachedSession())) {
      // Ohne Sitzung führt der gemeinsame Fehlerweg zur Anmeldung.
      throw new ApiError("Authentifizierung erforderlich", 401);
    }
    const formData = new FormData();
    formData.append("file", file);
    // Raw fetch on purpose: sessionFetch forces application/json and would
    // clobber the multipart boundary.
    const response = await transportFetch(
      `/api/students/${studentId}/accident-reports/${reportIdValue}/paper-copy`,
      { method: "POST", body: formData },
    );
    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        "Papierexemplar konnte nicht hochgeladen werden.",
      );
    }
  }

  /** Same-origin proxy URL of the report as PDF. */
  pdfUrl(studentId: string, reportIdValue: string): string {
    return `/api/students/${studentId}/accident-reports/${reportIdValue}/pdf`;
  }
}

export const studentAccidentReportsService =
  new StudentAccidentReportsService();
