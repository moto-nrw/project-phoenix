export interface InvitationValidation {
  email: string;
  roleName: string;
  firstName?: string | null;
  lastName?: string | null;
  position?: string | null;
  expiresAt: string;
}

export interface InvitationAcceptRequest {
  firstName: string;
  lastName: string;
  password: string;
  confirmPassword: string;
}

export interface CreateInvitationRequest {
  email: string;
  roleId: number | undefined;
  firstName?: string;
  lastName?: string;
  position?: string;
}

export type InvitationStatus =
  | "pending"
  | "failed"
  | "expired"
  | "accepted"
  | "revoked";

export type InvitationDeliveryStatus = "pending" | "sent" | "failed";

export interface InvitationRecord {
  id: number;
  email: string;
  roleId: number;
  roleName: string;
  status?: InvitationStatus;
  createdBy: number;
  creatorEmail?: string;
  createdAt?: string;
  expiresAt: string;
  usedAt?: string | null;
  revokedAt?: string | null;
  deliveryStatus?: InvitationDeliveryStatus;
  emailSentAt?: string | null;
  emailError?: string | null;
  emailRetryCount?: number;
  token?: string;
  firstName?: string | null;
  lastName?: string | null;
  position?: string | null;
}

export type PendingInvitation = InvitationRecord;

export interface BackendInvitationValidation {
  email: string;
  role_name: string;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  expires_at: string;
}

export interface BackendInvitation {
  id: number;
  email: string;
  role_id: number;
  role_name?: string; // Role name from backend
  status?: InvitationStatus;
  token?: string;
  created_at?: string;
  expires_at: string;
  used_at?: string | null;
  revoked_at?: string | null;
  created_by: number;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  creator?: string; // Creator email from backend
  delivery_status?: InvitationDeliveryStatus;
  email_sent_at?: string | null;
  email_error?: string | null;
  email_retry_count?: number;
}

export const mapInvitationValidationResponse = (
  data: BackendInvitationValidation,
): InvitationValidation => ({
  email: data.email,
  roleName: data.role_name,
  firstName: data.first_name,
  lastName: data.last_name,
  position: data.position,
  expiresAt: data.expires_at,
});

export const mapInvitationResponse = (
  data: BackendInvitation,
): InvitationRecord => {
  return {
    id: data.id,
    email: data.email,
    roleId: data.role_id,
    roleName: data.role_name ?? "",
    status: data.status ?? "pending",
    createdBy: data.created_by,
    creatorEmail: data.creator,
    createdAt: data.created_at ?? data.expires_at,
    expiresAt: data.expires_at,
    usedAt: data.used_at,
    revokedAt: data.revoked_at,
    deliveryStatus: data.delivery_status ?? "pending",
    emailSentAt: data.email_sent_at,
    emailError: data.email_error,
    emailRetryCount: data.email_retry_count ?? 0,
    token: data.token,
    firstName: data.first_name,
    lastName: data.last_name,
    position: data.position,
  };
};

export const mapPendingInvitationResponse = mapInvitationResponse;
