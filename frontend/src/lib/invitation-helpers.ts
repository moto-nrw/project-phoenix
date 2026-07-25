import { LOCATION_COLORS } from "~/lib/location-helper";

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

export interface PendingInvitation {
  id: number;
  email: string;
  roleId: number;
  roleName: string;
  createdBy: number;
  creatorEmail?: string;
  expiresAt: string;
  token?: string;
  firstName?: string | null;
  lastName?: string | null;
  position?: string | null;
  /**
   * Email delivery state the backend already tracks per invitation token
   * (#1937). It was being dropped here, so a staff invitation whose mail
   * failed looked identical to one that arrived.
   */
  deliveryStatus?: StaffInvitationDeliveryStatus;
  emailError?: string | null;
  emailRetryCount?: number;
}

/** Mirrors the backend's deriveDeliveryStatus. */
export type StaffInvitationDeliveryStatus = "pending" | "sent" | "failed";

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
  token?: string;
  expires_at: string;
  created_by: number;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  creator?: string; // Creator email from backend
  delivery_status?: StaffInvitationDeliveryStatus;
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

export const mapPendingInvitationResponse = (
  data: BackendInvitation,
): PendingInvitation => {
  return {
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
    deliveryStatus: data.delivery_status,
    emailError: data.email_error,
    emailRetryCount: data.email_retry_count,
  };
};

/**
 * Presentation for a staff invitation's email delivery state. "sent" means the
 * mail server accepted it — deliberately not "zugestellt", which we cannot
 * know for this flow (it does not go through the outbox/webhook path).
 */
export const STAFF_INVITATION_DELIVERY_META: Record<
  StaffInvitationDeliveryStatus,
  { label: string; color: string }
> = {
  pending: { label: "E-Mail eingeplant", color: LOCATION_COLORS.UNKNOWN },
  sent: { label: "E-Mail versendet", color: LOCATION_COLORS.GROUP_ROOM },
  failed: { label: "Versand fehlgeschlagen", color: LOCATION_COLORS.HOME },
};
