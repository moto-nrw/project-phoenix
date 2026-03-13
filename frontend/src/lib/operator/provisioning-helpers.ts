// Backend response types (snake_case, int64 ids)

export interface BackendOrganization {
  id: number;
  name: string;
  slug: string;
  active: boolean;
  settings: string | null;
  created_at: string;
  updated_at: string;
}

export interface BackendSchool {
  id: number;
  organization_id: number;
  name: string;
  slug: string;
  subdomain: string;
  address: string;
  city: string;
  zip: string;
  phone: string;
  email: string;
  active: boolean;
  settings: string | null;
  created_at: string;
  updated_at: string;
  organization?: BackendOrganization;
}

export interface BackendInvitation {
  id: number;
  email: string;
  role_id: number;
  role_name?: string;
  expires_at: string;
  first_name?: string | null;
  last_name?: string | null;
  position?: string | null;
  created_by: number;
  creator?: string;
  delivery_status: string;
  email_sent_at?: string | null;
  email_error?: string | null;
  email_retry_count: number;
}

// Frontend types (camelCase, string ids)

export interface Organization {
  id: string;
  name: string;
  slug: string;
  active: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface School {
  id: string;
  organizationId: string;
  name: string;
  slug: string;
  subdomain: string;
  address: string;
  city: string;
  zip: string;
  phone: string;
  email: string;
  active: boolean;
  createdAt: string;
  updatedAt: string;
  organization?: Organization;
}

export interface Invitation {
  id: string;
  email: string;
  roleId: string;
  roleName: string;
  expiresAt: string;
  firstName: string | null;
  lastName: string | null;
  position: string | null;
  createdBy: string;
  creator: string;
  deliveryStatus: string;
  emailSentAt: string | null;
  emailError: string | null;
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

// Mapping functions

export function mapOrganization(data: BackendOrganization): Organization {
  return {
    id: data.id.toString(),
    name: data.name,
    slug: data.slug,
    active: data.active,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

export function mapSchool(data: BackendSchool): School {
  return {
    id: data.id.toString(),
    organizationId: data.organization_id.toString(),
    name: data.name,
    slug: data.slug,
    subdomain: data.subdomain,
    address: data.address,
    city: data.city,
    zip: data.zip,
    phone: data.phone,
    email: data.email,
    active: data.active,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
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
    roleName: data.role_name ?? "",
    expiresAt: data.expires_at,
    firstName: data.first_name ?? null,
    lastName: data.last_name ?? null,
    position: data.position ?? null,
    createdBy: data.created_by.toString(),
    creator: data.creator ?? "",
    deliveryStatus: data.delivery_status,
    emailSentAt: data.email_sent_at ?? null,
    emailError: data.email_error ?? null,
    emailRetryCount: data.email_retry_count,
  };
}

// Slug helpers

const SLUG_REGEX = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

export function generateSlug(name: string): string {
  return name
    .toLowerCase()
    .replace(/[äÄ]/g, "ae")
    .replace(/[öÖ]/g, "oe")
    .replace(/[üÜ]/g, "ue")
    .replace(/[ß]/g, "ss")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .replace(/-{2,}/g, "-");
}

export function isValidSlug(slug: string): boolean {
  return SLUG_REGEX.test(slug);
}
