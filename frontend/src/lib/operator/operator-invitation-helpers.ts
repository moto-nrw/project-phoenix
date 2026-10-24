// --- Backend response types (snake_case) ---

export interface BackendInvitationResponse {
  id: number;
  email: string;
  display_name?: string | null;
  created_by: number;
  creator_name?: string;
  expires_at: string;
  email_sent_at?: string | null;
  email_error?: string | null;
  email_retry_count: number;
  created_at: string;
}

export interface BackendOperatorListItem {
  id: number;
  email: string;
  display_name: string;
  active: boolean;
  last_login?: string | null;
  deactivated_at?: string | null;
  deletion_due_at?: string | null;
  created_at: string;
  is_self: boolean;
}

export interface BackendInvitationsListResponse {
  invitations: BackendInvitationResponse[];
  operators: BackendOperatorListItem[];
}

export interface BackendInvitationValidation {
  email: string;
  display_name?: string | null;
  expires_at: string;
}

// --- Frontend types (camelCase) ---

export interface PendingOperatorInvitation {
  id: string;
  email: string;
  displayName?: string;
  createdBy: string;
  creatorName?: string;
  expiresAt: string;
  emailSentAt?: string;
  emailError?: string;
  emailRetryCount: number;
  createdAt: string;
}

export interface OperatorInfo {
  id: string;
  email: string;
  displayName: string;
  active: boolean;
  lastLogin?: string;
  /** Set while the account is deactivated; the account is deleted later. */
  deactivatedAt?: string;
  /** When moto deletes the deactivated account. */
  deletionDueAt?: string;
  createdAt: string;
  /** The signed-in operator, who cannot deactivate their own account. */
  isSelf: boolean;
}

export interface OperatorInvitationValidation {
  email: string;
  displayName?: string;
  expiresAt: string;
}

export interface CreateOperatorInvitationRequest {
  email: string;
  displayName?: string;
}

export interface AcceptOperatorInvitationRequest {
  displayName: string;
  password: string;
  confirmPassword: string;
}

// --- Mappers ---

export function mapPendingInvitation(
  data: BackendInvitationResponse,
): PendingOperatorInvitation {
  return {
    id: data.id.toString(),
    email: data.email,
    displayName: data.display_name ?? undefined,
    createdBy: data.created_by.toString(),
    creatorName: data.creator_name,
    expiresAt: data.expires_at,
    emailSentAt: data.email_sent_at ?? undefined,
    emailError: data.email_error ?? undefined,
    emailRetryCount: data.email_retry_count,
    createdAt: data.created_at,
  };
}

export function mapOperatorInfo(data: BackendOperatorListItem): OperatorInfo {
  return {
    id: data.id.toString(),
    email: data.email,
    displayName: data.display_name,
    active: data.active,
    lastLogin: data.last_login ?? undefined,
    deactivatedAt: data.deactivated_at ?? undefined,
    deletionDueAt: data.deletion_due_at ?? undefined,
    createdAt: data.created_at,
    isSelf: data.is_self,
  };
}

export function mapInvitationValidation(
  data: BackendInvitationValidation,
): OperatorInvitationValidation {
  return {
    email: data.email,
    displayName: data.display_name ?? undefined,
    expiresAt: data.expires_at,
  };
}
