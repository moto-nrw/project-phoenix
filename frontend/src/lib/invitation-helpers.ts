export interface InvitationValidation {
  targetPortal?: "tenant" | "school";
  requiresAccountLogin?: boolean;
  email: string;
  roleName: string;
  firstName?: string | null;
  lastName?: string | null;
  position?: string | null;
  expiresAt: string;
}

export interface InvitationAcceptRequest {
  existingAccount?: boolean;
  firstName: string;
  lastName: string;
  password: string;
  confirmPassword: string;
}

export type EmploymentAction = "resume" | "new";

export interface CreateInvitationRequest {
  employmentAction?: EmploymentAction;
  email: string;
  roleId: string | undefined;
  firstName?: string;
  lastName?: string;
  position?: string;
}

export interface PendingInvitation {
  employmentAction?: EmploymentAction;
  id: number;
  email: string;
  roleId: string;
  roleName: string;
  createdBy: number;
  creatorEmail?: string;
  expiresAt: string;
  token?: string;
  firstName?: string | null;
  lastName?: string | null;
  position?: string | null;
}

export interface BackendInvitationValidation {
  target_portal?: "tenant" | "school";
  requires_account_login?: boolean;
  email: string;
  role_name: string;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  expires_at: string;
}

export interface BackendInvitation {
  employment_action?: EmploymentAction;
  id: number;
  email: string;
  role_id: string;
  role_name?: string; // Role name from backend
  token?: string;
  expires_at: string;
  created_by: number;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  creator?: string; // Creator email from backend
}

export const mapInvitationValidationResponse = (
  data: BackendInvitationValidation,
): InvitationValidation => ({
  targetPortal: data.target_portal,
  requiresAccountLogin: data.requires_account_login,
  email: data.email,
  roleName: data.role_name,
  firstName: data.first_name,
  lastName: data.last_name,
  position: data.position,
  expiresAt: data.expires_at,
});

export const mapPendingInvitationResponse = (
  data: BackendInvitation,
): PendingInvitation => {
  return {
    employmentAction: data.employment_action,
    id: data.id,
    email: data.email,
    roleId: data.role_id,
    roleName: data.role_name ?? "",
    createdBy: data.created_by,
    creatorEmail: data.creator,
    expiresAt: data.expires_at,
    token: data.token,
    firstName: data.first_name,
    lastName: data.last_name,
    position: data.position,
  };
};

/**
 * The parts of a former employment the backend names (ADR 0021): what a
 * Wiederaufnahme brings back and what never returns.
 */
export const FORMER_EMPLOYMENT_PART_LABELS = {
  master_data: "Stammdaten",
  qualifications: "Qualifikationen",
  work_time: "Stundenkonto, Urlaub und Arbeitszeitmodell",
  past_shifts_and_absences: "vergangene Schichten und Abwesenheiten",
  staff_documents: "Dokumente",
  future_shifts_and_absences: "künftige Schichten und Abwesenheiten",
  groups_classes_substitutions: "Gruppen, Klassen und Vertretungen",
} as const;

export type FormerEmploymentPart = keyof typeof FORMER_EMPLOYMENT_PART_LABELS;

/**
 * The former employment an invitation to an address would continue at this
 * school (ADR 0021, #3402). Dates are calendar days ("YYYY-MM-DD"): the days
 * the person was entered in moto and left.
 */
export interface FormerEmployment {
  startedOn: string;
  endedOn: string;
  resumes: FormerEmploymentPart[];
  notReturning: FormerEmploymentPart[];
}

export interface BackendFormerEmployment {
  started_on: string;
  ended_on: string;
  resumes: string[];
  not_returning: string[];
}

const isFormerEmploymentPart = (part: string): part is FormerEmploymentPart =>
  Object.hasOwn(FORMER_EMPLOYMENT_PART_LABELS, part);

/** A part key this frontend does not know yet is left out, not shown raw. */
export const mapFormerEmploymentResponse = (
  data: BackendFormerEmployment | null | undefined,
): FormerEmployment | null =>
  data
    ? {
        startedOn: data.started_on,
        endedOn: data.ended_on,
        resumes: data.resumes.filter(isFormerEmploymentPart),
        notReturning: data.not_returning.filter(isFormerEmploymentPart),
      }
    : null;
