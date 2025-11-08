export interface InvitationValidation {
  email: string;
  roleName: string;
  firstName?: string | null;
  lastName?: string | null;
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
}

export interface BackendInvitationValidation {
  email: string;
  role_name: string;
  first_name?: string | null;
  last_name?: string | null;
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
  creator?: string; // Creator email from backend
}

export const mapInvitationValidationResponse = (
  data: BackendInvitationValidation,
): InvitationValidation => ({
  email: data.email,
  roleName: data.role_name,
  firstName: data.first_name,
  lastName: data.last_name,
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
  };
};
