// Backend response types (snake_case, int64 ids)

export interface BackendProvisioningStats {
  traeger_count: number;
  schulen_count: number;
  konten_count: number;
  geraete_count: number;
}

export interface ProvisioningStats {
  traegerCount: number;
  schulenCount: number;
  kontenCount: number;
  geraeteCount: number;
}

export function mapProvisioningStats(
  data: BackendProvisioningStats,
): ProvisioningStats {
  return {
    traegerCount: data.traeger_count,
    schulenCount: data.schulen_count,
    kontenCount: data.konten_count,
    geraeteCount: data.geraete_count,
  };
}

export interface BackendOrganizationSummary extends BackendOrganization {
  schulen_count: number;
  konten_count: number;
  geraete_count: number;
  personen_count: number;
}

export interface OrganizationSummary extends Organization {
  schulenCount: number;
  kontenCount: number;
  geraeteCount: number;
  personenCount: number;
}

export function mapOrganizationSummary(
  data: BackendOrganizationSummary,
): OrganizationSummary {
  return {
    ...mapOrganization(data),
    schulenCount: data.schulen_count,
    kontenCount: data.konten_count,
    geraeteCount: data.geraete_count,
    personenCount: data.personen_count,
  };
}

export interface BackendSchoolSummary {
  id: number;
  organization_id: number;
  organization_name: string;
  name: string;
  slug: string;
  subdomain: string;
  active: boolean;
  hidden: boolean;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
  address: string;
  city: string;
  zip: string;
  phone: string;
  email: string;
  settings: string | null;
  konten_count: number;
  geraete_count: number;
  personen_count: number;
}

export interface SchoolSummary {
  id: string;
  organizationId: string;
  organizationName: string;
  name: string;
  slug: string;
  subdomain: string;
  active: boolean;
  hidden: boolean;
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
  address: string;
  city: string;
  zip: string;
  phone: string;
  email: string;
  kontenCount: number;
  geraeteCount: number;
  personenCount: number;
}

export function mapSchoolSummary(data: BackendSchoolSummary): SchoolSummary {
  return {
    id: data.id.toString(),
    organizationId: data.organization_id.toString(),
    organizationName: data.organization_name,
    name: data.name,
    slug: data.slug,
    subdomain: data.subdomain,
    active: data.active,
    hidden: data.hidden,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    deletedAt: data.deleted_at ?? null,
    address: data.address,
    city: data.city,
    zip: data.zip,
    phone: data.phone,
    email: data.email,
    kontenCount: data.konten_count,
    geraeteCount: data.geraete_count,
    personenCount: data.personen_count,
  };
}

// Adapters: drop the summary-only count fields so existing modal/form
// components that accept the base Organization/School types can be reused on
// drill-in pages without leaking aggregate counts into their props.
export function summaryToOrganization(
  summary: OrganizationSummary,
): Organization {
  return {
    id: summary.id,
    name: summary.name,
    slug: summary.slug,
    active: summary.active,
    deletedAt: summary.deletedAt,
    createdAt: summary.createdAt,
    updatedAt: summary.updatedAt,
  };
}

export function summaryToSchool(summary: SchoolSummary): School {
  return {
    id: summary.id,
    organizationId: summary.organizationId,
    name: summary.name,
    slug: summary.slug,
    subdomain: summary.subdomain,
    address: summary.address,
    city: summary.city,
    zip: summary.zip,
    phone: summary.phone,
    email: summary.email,
    active: summary.active,
    hidden: summary.hidden,
    deletedAt: summary.deletedAt,
    createdAt: summary.createdAt,
    updatedAt: summary.updatedAt,
  };
}

export interface BackendOrganization {
  id: number;
  name: string;
  slug: string;
  active: boolean;
  // Backend uses json:",omitempty" on a *time.Time, so the field is absent
  // (undefined) when the row is not soft-deleted, not null.
  deleted_at?: string | null;
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
  hidden: boolean;
  // Backend uses json:",omitempty" on a *time.Time, so the field is absent
  // (undefined) when the row is not soft-deleted, not null.
  deleted_at?: string | null;
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
  caregiver_enabled?: boolean;
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
  deletedAt: string | null;
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
  hidden: boolean;
  deletedAt: string | null;
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
  caregiverEnabled: boolean;
  createdBy: string;
  creator: string;
  deliveryStatus: string;
  emailSentAt: string | null;
  emailError: string | null;
  emailRetryCount: number;
}

// Account types for operator school accounts listing

export interface BackendSchoolAccount {
  account_id: number;
  email: string;
  active: boolean;
  first_name: string;
  last_name: string;
  role_name: string;
  pedagogic_role: string;
  status: string;
  has_admin_role?: boolean;
  has_user_role?: boolean;
  has_caregiver_profile?: boolean;
  is_active_caregiver?: boolean;
}

export interface SchoolAccount {
  accountId: string;
  email: string;
  active: boolean;
  firstName: string;
  lastName: string;
  roleName: string;
  pedagogicRole: string;
  status: string;
  hasAdminRole: boolean;
  hasUserRole: boolean;
  hasCaregiverProfile: boolean;
  isActiveCaregiver: boolean;
}

export function mapSchoolAccount(data: BackendSchoolAccount): SchoolAccount {
  return {
    accountId: data.account_id.toString(),
    email: data.email,
    active: data.active,
    firstName: data.first_name,
    lastName: data.last_name,
    roleName: data.role_name,
    pedagogicRole: data.pedagogic_role,
    status: data.status,
    hasAdminRole: data.has_admin_role ?? false,
    hasUserRole: data.has_user_role ?? false,
    hasCaregiverProfile: data.has_caregiver_profile ?? false,
    isActiveCaregiver: data.is_active_caregiver ?? false,
  };
}

// Org-level account types (includes school context)

export interface BackendOrgAccount extends BackendSchoolAccount {
  school_id: number;
  school_name: string;
}

export interface OrgAccount extends SchoolAccount {
  schoolId: string;
  schoolName: string;
}

export function mapOrgAccount(data: BackendOrgAccount): OrgAccount {
  return {
    ...mapSchoolAccount(data),
    schoolId: data.school_id.toString(),
    schoolName: data.school_name,
  };
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
  hidden?: boolean;
}

export interface InviteAdminRequest {
  email: string;
  first_name?: string;
  last_name?: string;
  position?: string;
  caregiver_enabled?: boolean;
}

export interface CreateAccountRequest {
  email: string;
  first_name: string;
  last_name: string;
  password: string;
  confirm_password: string;
  role_id?: number;
  position?: string;
  caregiver_enabled?: boolean;
}

// Mapping functions

export function mapOrganization(data: BackendOrganization): Organization {
  return {
    id: data.id.toString(),
    name: data.name,
    slug: data.slug,
    active: data.active,
    deletedAt: data.deleted_at ?? null,
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
    hidden: data.hidden,
    deletedAt: data.deleted_at ?? null,
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
    caregiverEnabled: data.caregiver_enabled ?? false,
    createdBy: data.created_by.toString(),
    creator: data.creator ?? "",
    deliveryStatus: data.delivery_status,
    emailSentAt: data.email_sent_at ?? null,
    emailError: data.email_error ?? null,
    emailRetryCount: data.email_retry_count,
  };
}

export interface UpdateOrganizationRequest {
  name: string;
  slug: string;
  active: boolean;
}

export interface UpdateSchoolRequest {
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
  hidden: boolean;
}

// Device creation / API key management

export interface CreateDeviceRequest {
  school_id: number;
  device_id: string;
  device_type: string;
  name?: string;
  api_key?: string;
}

// Device types for operator device listing

export interface BackendOperatorDevice {
  id: number;
  device_id: string;
  device_type: string;
  name?: string;
  status: string;
  api_key?: string;
  masked_api_key: string;
  last_seen?: string;
  is_online: boolean;
  school_id: number;
  school_name: string;
  organization_id: number;
  organization_name: string;
  created_at: string;
  updated_at: string;
}

export interface OperatorDevice {
  id: string;
  deviceId: string;
  deviceType: string;
  name: string;
  status: string;
  apiKey: string;
  maskedApiKey: string;
  lastSeen: string | null;
  isOnline: boolean;
  schoolId: string;
  schoolName: string;
  organizationId: string;
  organizationName: string;
  createdAt: string;
  updatedAt: string;
}

export function mapOperatorDevice(data: BackendOperatorDevice): OperatorDevice {
  return {
    id: data.id.toString(),
    deviceId: data.device_id,
    deviceType: data.device_type,
    name: data.name ?? "",
    status: data.status,
    apiKey: data.api_key ?? "",
    maskedApiKey: data.masked_api_key,
    lastSeen: data.last_seen ?? null,
    isOnline: data.is_online,
    schoolId: data.school_id.toString(),
    schoolName: data.school_name,
    organizationId: data.organization_id.toString(),
    organizationName: data.organization_name,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

interface BackendDeviceTransferSession {
  id: number;
  started_at: string;
  activity_name?: string;
  room_name?: string;
}

export interface BackendDeviceTransferStatus {
  can_transfer: boolean;
  is_online: boolean;
  is_protected: boolean;
  last_seen?: string;
  active_session?: BackendDeviceTransferSession;
}

export interface DeviceTransferStatus {
  canTransfer: boolean;
  isOnline: boolean;
  isProtected: boolean;
  lastSeen: string | null;
  activeSession: {
    id: string;
    startedAt: string;
    activityName: string;
    roomName: string;
  } | null;
}

export function mapDeviceTransferStatus(
  data: BackendDeviceTransferStatus,
): DeviceTransferStatus {
  return {
    canTransfer: data.can_transfer,
    isOnline: data.is_online,
    isProtected: data.is_protected,
    lastSeen: data.last_seen ?? null,
    activeSession: data.active_session
      ? {
          id: data.active_session.id.toString(),
          startedAt: data.active_session.started_at,
          activityName: data.active_session.activity_name ?? "",
          roomName: data.active_session.room_name ?? "",
        }
      : null,
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

// Person types for operator person listing + soft delete

export interface BackendOperatorPerson {
  id: number;
  first_name: string;
  last_name: string;
  has_account: boolean;
  account_email?: string | null;
  has_rfid_card: boolean;
  is_staff: boolean;
  is_student: boolean;
  school_id: number;
  school_name: string;
  organization_id: number;
  organization_name: string;
  created_at: string;
}

export interface OperatorPerson {
  id: string;
  firstName: string;
  lastName: string;
  fullName: string;
  hasAccount: boolean;
  accountEmail: string | null;
  hasRfidCard: boolean;
  isStaff: boolean;
  isStudent: boolean;
  schoolId: string;
  schoolName: string;
  organizationId: string;
  organizationName: string;
  createdAt: string;
}

export function mapOperatorPerson(data: BackendOperatorPerson): OperatorPerson {
  return {
    id: data.id.toString(),
    firstName: data.first_name,
    lastName: data.last_name,
    fullName: `${data.first_name} ${data.last_name}`,
    hasAccount: data.has_account,
    accountEmail: data.account_email ?? null,
    hasRfidCard: data.has_rfid_card,
    isStaff: data.is_staff,
    isStudent: data.is_student,
    schoolId: data.school_id.toString(),
    schoolName: data.school_name,
    organizationId: data.organization_id.toString(),
    organizationName: data.organization_name,
    createdAt: data.created_at,
  };
}

export interface BackendUnregisteredTagScan {
  id: number;
  tenant_id: number;
  tag_uid: string;
  device_id?: number | null;
  scanned_at: string;
  resolved_at?: string | null;
  resolved_by_operator_id?: number | null;
  resolution_note?: string | null;
  created_at: string;
  updated_at: string;
  school_id: number;
  school_name: string;
  organization_id: number;
  organization_name: string;
  device_identifier?: string | null;
  device_name?: string | null;
}

export interface UnregisteredTagScan {
  id: string;
  tenantId: string;
  tagUid: string;
  deviceId: string | null;
  scannedAt: string;
  resolvedAt: string | null;
  resolvedByOperatorId: string | null;
  resolutionNote: string | null;
  createdAt: string;
  updatedAt: string;
  schoolId: string;
  schoolName: string;
  organizationId: string;
  organizationName: string;
  deviceIdentifier: string | null;
  deviceName: string | null;
}

export function mapUnregisteredTagScan(
  data: BackendUnregisteredTagScan,
): UnregisteredTagScan {
  return {
    id: data.id.toString(),
    tenantId: data.tenant_id.toString(),
    tagUid: data.tag_uid,
    deviceId: data.device_id != null ? data.device_id.toString() : null,
    scannedAt: data.scanned_at,
    resolvedAt: data.resolved_at ?? null,
    resolvedByOperatorId:
      data.resolved_by_operator_id != null
        ? data.resolved_by_operator_id.toString()
        : null,
    resolutionNote: data.resolution_note ?? null,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    schoolId: data.school_id.toString(),
    schoolName: data.school_name,
    organizationId: data.organization_id.toString(),
    organizationName: data.organization_name,
    deviceIdentifier: data.device_identifier ?? null,
    deviceName: data.device_name ?? null,
  };
}
