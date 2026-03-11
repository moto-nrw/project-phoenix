// Backend types (snake_case, number IDs)

export interface BackendOrganization {
  id: number;
  created_at: string;
  updated_at: string;
  name: string;
  slug: string;
  active: boolean;
  settings?: string;
}

export interface BackendSchool {
  id: number;
  created_at: string;
  updated_at: string;
  organization_id: number;
  name: string;
  slug: string;
  subdomain: string;
  active: boolean;
  settings?: string;
  address?: string;
  city?: string;
  zip?: string;
  phone?: string;
  email?: string;
  organization?: BackendOrganization;
}

export interface BackendInvitation {
  id: number;
  email: string;
  role_id: number;
  role_name?: string;
  expires_at: string;
  first_name?: string;
  last_name?: string;
  position?: string;
  created_by: number;
  creator?: string;
  delivery_status: string;
  email_sent_at?: string;
  email_error?: string;
  email_retry_count: number;
}

// Frontend types (camelCase, string IDs)

export interface Organization {
  id: string;
  createdAt: string;
  updatedAt: string;
  name: string;
  slug: string;
  active: boolean;
}

export interface School {
  id: string;
  createdAt: string;
  updatedAt: string;
  organizationId: string;
  name: string;
  slug: string;
  subdomain: string;
  active: boolean;
  address?: string;
  city?: string;
  zip?: string;
  phone?: string;
  email?: string;
  organization?: Organization;
}

export interface Invitation {
  id: string;
  email: string;
  roleId: string;
  roleName?: string;
  expiresAt: string;
  firstName?: string;
  lastName?: string;
  position?: string;
  createdBy: string;
  creator?: string;
  deliveryStatus: string;
  emailSentAt?: string;
  emailError?: string;
  emailRetryCount: number;
}

// Request types (snake_case for backend)

export interface CreateOrganizationRequest {
  name: string;
  slug: string;
}

export interface CreateSchoolRequest {
  organization_id: number;
  name: string;
  slug: string;
  subdomain: string;
  address?: string;
  city?: string;
  zip?: string;
  phone?: string;
  email?: string;
}

export interface InviteAdminRequest {
  email: string;
  first_name?: string;
  last_name?: string;
  position?: string;
}

// Mappers

export function mapOrganization(data: BackendOrganization): Organization {
  return {
    id: data.id.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    name: data.name,
    slug: data.slug,
    active: data.active,
  };
}

export function mapSchool(data: BackendSchool): School {
  return {
    id: data.id.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    organizationId: data.organization_id.toString(),
    name: data.name,
    slug: data.slug,
    subdomain: data.subdomain,
    active: data.active,
    address: data.address,
    city: data.city,
    zip: data.zip,
    phone: data.phone,
    email: data.email,
    organization: data.organization
      ? mapOrganization(data.organization)
      : undefined,
  };
}

export function mapInvitation(data: BackendInvitation): Invitation {
  return {
    id: data.id.toString(),
    email: data.email,
    roleId: data.role_id.toString(),
    roleName: data.role_name,
    expiresAt: data.expires_at,
    firstName: data.first_name,
    lastName: data.last_name,
    position: data.position,
    createdBy: data.created_by.toString(),
    creator: data.creator,
    deliveryStatus: data.delivery_status,
    emailSentAt: data.email_sent_at,
    emailError: data.email_error,
    emailRetryCount: data.email_retry_count,
  };
}

// Validation

export const SLUG_REGEX = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

export function generateSlug(name: string): string {
  return name
    .toLowerCase()
    .replace(/[äÄ]/g, "ae")
    .replace(/[öÖ]/g, "oe")
    .replace(/[üÜ]/g, "ue")
    .replace(/[ß]/g, "ss")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 100);
}

export function validateSlug(slug: string): string | null {
  if (!slug) return "Slug ist erforderlich";
  if (slug.length > 100) return "Slug darf maximal 100 Zeichen haben";
  if (!SLUG_REGEX.test(slug))
    return "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt";
  return null;
}

export function validateSubdomain(subdomain: string): string | null {
  if (!subdomain) return "Subdomain ist erforderlich";
  if (subdomain.length > 63) return "Subdomain darf maximal 63 Zeichen haben";
  if (!SLUG_REGEX.test(subdomain))
    return "Nur Kleinbuchstaben, Zahlen und Bindestriche erlaubt";
  return null;
}

// Conflict matchers (parse 409 error messages from backend)

export function isSlugConflict(errorMessage: string): boolean {
  return errorMessage.toLowerCase().includes("slug");
}

export function isSubdomainConflict(errorMessage: string): boolean {
  return errorMessage.toLowerCase().includes("subdomain");
}

export function isEmailConflict(errorMessage: string): boolean {
  return errorMessage.toLowerCase().includes("email");
}

// German labels for delivery status

export type DeliveryStatus = "sent" | "failed" | "pending";

export const DELIVERY_STATUS_LABELS: Record<DeliveryStatus, string> = {
  sent: "Gesendet",
  failed: "Fehlgeschlagen",
  pending: "Ausstehend",
};

export const DELIVERY_STATUS_STYLES: Record<DeliveryStatus, string> = {
  sent: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
  pending: "bg-yellow-100 text-yellow-700",
};
