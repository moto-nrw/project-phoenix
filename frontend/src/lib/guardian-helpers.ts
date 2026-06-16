// Guardian Type Definitions and Mapping Helpers

// Phone Number Types
export type PhoneType = "mobile" | "home" | "work" | "other";

export interface PhoneNumber {
  id: string;
  phoneNumber: string;
  phoneType: PhoneType;
  label?: string;
  isPrimary: boolean;
  priority: number;
}

export interface BackendPhoneNumber {
  id: number;
  phone_number: string;
  phone_type: PhoneType;
  label?: string;
  is_primary: boolean;
  priority: number;
}

// Phone number type labels (German)
export const PHONE_TYPE_LABELS: Record<PhoneType, string> = {
  mobile: "Mobil",
  home: "Telefon",
  work: "Dienstlich",
  other: "Sonstige",
};

export type GuardianRole =
  | "primary_guardian"
  | "legal_guardian"
  | "co_guardian"
  | "emergency_contact"
  | "pickup_only"
  | "social_worker"
  | "custom";

export const GUARDIAN_ROLE_OPTIONS: Array<{
  value: GuardianRole;
  label: string;
}> = [
  { value: "primary_guardian", label: "Hauptberechtigte/r" },
  { value: "legal_guardian", label: "Erziehungsberechtigte/r" },
  { value: "co_guardian", label: "Mitberechtigte/r" },
  { value: "emergency_contact", label: "Notfallkontakt" },
  { value: "pickup_only", label: "Nur Abholung" },
  { value: "social_worker", label: "Sozialdienst" },
  { value: "custom", label: "Individuell" },
];

function normalizeGuardianRole(value: unknown): GuardianRole {
  return GUARDIAN_ROLE_OPTIONS.some((option) => option.value === value)
    ? (value as GuardianRole)
    : "custom";
}

// Frontend Guardian Profile Type
export interface Guardian {
  id: string;
  firstName: string;
  lastName: string;
  email?: string;
  phoneNumbers: PhoneNumber[]; // Flexible phone numbers
  addressStreet?: string;
  addressCity?: string;
  addressPostalCode?: string;
  preferredContactMethod: string;
  languagePreference: string;
  notes?: string;
  hasAccount: boolean;
  accountId?: string;
  // Populated only by the guardian picker search (GET /guardians/search). The
  // picker is open to all staff, so it never carries the linked children's
  // names — only how many other children the guardian is linked to, for
  // disambiguation (#1513 GDPR minimization).
  linkedChildrenCount?: number;
}

// Backend Guardian Profile Response
export interface BackendGuardianProfile {
  id: number;
  first_name: string;
  last_name: string;
  email?: string;
  phone_numbers?: BackendPhoneNumber[]; // Flexible phone numbers
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  preferred_contact_method: string;
  language_preference: string;
  notes?: string;
  has_account: boolean;
  account_id?: number;
}

// Backend Guardian Picker Response (GET /guardians/search). This is the
// deliberately minimal, enumeration-resistant projection the picker endpoint
// emits — NOT a full profile. Address, notes, language, contact method,
// account, and phone numbers are all withheld server-side; and only a COUNT of
// other linked children is exposed, never their names (#1513 GDPR
// minimization). Kept separate from BackendGuardianProfile so callers can't
// accidentally read fields the picker never sends.
export interface BackendGuardianPickerResponse {
  id: number;
  first_name: string;
  last_name: string;
  email?: string;
  linked_children_count: number;
}

// Student-Guardian Relationship

// Backend Student-Guardian Relationship

// Portal-access state of a guardian, surfaced in the staff guardian list:
// "active" = has a login account · "pending" = invited, not yet accepted ·
// "none" = info on file, no account (can be invited).
export type GuardianAccountStatus = "active" | "pending" | "none";

// Guardian with Relationship (for student detail view)
export interface GuardianWithRelationship extends Guardian {
  relationshipId: string;
  relationshipType: string;
  guardianRole?: GuardianRole;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  pickupNotes?: string;
  emergencyPriority: number;
  // Optional: present on data mapped from the backend (always set by
  // mapGuardianWithRelationshipResponse), but omittable in test fixtures and
  // older payloads. Consumers default a missing value to "none".
  accountStatus?: GuardianAccountStatus;
}

// Backend Guardian with Relationship
export interface BackendGuardianWithRelationship {
  guardian: BackendGuardianProfile;
  relationship_id: number;
  relationship_type: string;
  guardian_role?: string;
  is_primary: boolean;
  is_emergency_contact: boolean;
  can_pickup: boolean;
  pickup_notes?: string;
  emergency_priority: number;
  account_status?: GuardianAccountStatus;
}

// Guardian Create/Update Request
export interface GuardianFormData {
  firstName: string;
  lastName: string;
  email?: string;
  addressStreet?: string;
  addressCity?: string;
  addressPostalCode?: string;
  preferredContactMethod?: string;
  languagePreference?: string;
  notes?: string;
}

// Snake-case guardian payload sent together with a new student to
// POST /api/students (backend `GuardianInput`). Used when creating a child and
// its guardians in one atomic request, mirroring the guardian fields managed on
// the student detail page.
export interface StudentGuardianPayload {
  // When set, links an EXISTING guardian profile to the new student instead of
  // creating one (sibling case, #1513). The profile fields below are then
  // carried only for local display in the create modal's summary list — the
  // backend ignores them and never mutates the existing profile.
  guardian_profile_id?: number;
  first_name: string;
  last_name: string;
  email?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  preferred_contact_method?: string;
  language_preference?: string;
  notes?: string;
  relationship_type: string;
  guardian_role?: GuardianRole;
  is_primary: boolean;
  is_emergency_contact: boolean;
  can_pickup: boolean;
  pickup_notes?: string;
  emergency_priority: number;
  phone_numbers: Array<{
    phone_number: string;
    phone_type: PhoneType;
    label?: string;
    is_primary: boolean;
  }>;
}

// Backend Guardian Create Request
// preferred_contact_method and language_preference are optional — the backend
// applies its own defaults ("phone" and "de") when omitted.
interface BackendGuardianCreateRequest {
  first_name: string;
  last_name: string;
  email?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  preferred_contact_method?: string;
  language_preference?: string;
  notes?: string;
}

// Student-Guardian Link Request
export interface StudentGuardianLinkRequest {
  guardianProfileId: string;
  relationshipType: string;
  guardianRole?: GuardianRole;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  pickupNotes?: string;
  emergencyPriority: number;
}

// Backend Student-Guardian Link Request
interface BackendStudentGuardianLinkRequest {
  guardian_profile_id: number;
  relationship_type: string;
  guardian_role?: GuardianRole;
  is_primary: boolean;
  is_emergency_contact: boolean;
  can_pickup: boolean;
  pickup_notes?: string;
  emergency_priority: number;
}

// Mapping Functions

// Map backend phone number to frontend format
export function mapPhoneNumberResponse(data: BackendPhoneNumber): PhoneNumber {
  return {
    id: data.id.toString(),
    phoneNumber: data.phone_number,
    phoneType: data.phone_type,
    label: data.label,
    isPrimary: data.is_primary,
    priority: data.priority,
  };
}

export function mapGuardianResponse(data: BackendGuardianProfile): Guardian {
  return {
    id: data.id.toString(),
    firstName: data.first_name,
    lastName: data.last_name,
    email: data.email,
    phoneNumbers: (data.phone_numbers ?? []).map(mapPhoneNumberResponse),
    addressStreet: data.address_street,
    addressCity: data.address_city,
    addressPostalCode: data.address_postal_code,
    preferredContactMethod: data.preferred_contact_method,
    languagePreference: data.language_preference,
    notes: data.notes,
    hasAccount: data.has_account,
    accountId: data.account_id?.toString(),
  };
}

// Maps the minimal guardian picker projection (GET /guardians/search) onto the
// Guardian shape used by the picker UI. The picker withholds most profile fields
// server-side, so the absent ones are filled with safe, explicit defaults rather
// than left undefined: phoneNumbers is empty (the picker never loads them) and
// hasAccount is false. Only id, name, email, and the linked-children COUNT carry
// real data. Use this — never mapGuardianResponse — for picker results, so
// nothing downstream trusts a field the picker never sent.
export function mapGuardianPickerResponse(
  data: BackendGuardianPickerResponse,
): Guardian {
  return {
    id: data.id.toString(),
    firstName: data.first_name,
    lastName: data.last_name,
    email: data.email,
    phoneNumbers: [],
    preferredContactMethod: "",
    languagePreference: "",
    hasAccount: false,
    linkedChildrenCount: data.linked_children_count,
  };
}

export function mapGuardianWithRelationshipResponse(
  data: BackendGuardianWithRelationship,
): GuardianWithRelationship {
  return {
    ...mapGuardianResponse(data.guardian),
    relationshipId: data.relationship_id.toString(),
    relationshipType: data.relationship_type,
    guardianRole: normalizeGuardianRole(data.guardian_role),
    isPrimary: data.is_primary,
    isEmergencyContact: data.is_emergency_contact,
    canPickup: data.can_pickup,
    pickupNotes: data.pickup_notes,
    emergencyPriority: data.emergency_priority,
    // Fall back to deriving from has_account if the backend omits the field
    // (older builds): account → active, otherwise none.
    accountStatus:
      data.account_status ?? (data.guardian.has_account ? "active" : "none"),
  };
}

export function mapGuardianFormDataToBackend(
  data: GuardianFormData,
): BackendGuardianCreateRequest {
  return {
    first_name: data.firstName,
    last_name: data.lastName,
    email: data.email,
    address_street: data.addressStreet,
    address_city: data.addressCity,
    address_postal_code: data.addressPostalCode,
    preferred_contact_method: data.preferredContactMethod,
    language_preference: data.languagePreference,
    notes: data.notes,
  };
}

export function mapStudentGuardianLinkToBackend(
  data: StudentGuardianLinkRequest,
): BackendStudentGuardianLinkRequest {
  return {
    guardian_profile_id: Number.parseInt(data.guardianProfileId),
    relationship_type: data.relationshipType,
    guardian_role: data.guardianRole,
    is_primary: data.isPrimary,
    is_emergency_contact: data.isEmergencyContact,
    can_pickup: data.canPickup,
    pickup_notes: data.pickupNotes,
    emergency_priority: data.emergencyPriority,
  };
}

// Relationship type options
export const RELATIONSHIP_TYPES = [
  { value: "parent", label: "Elternteil" },
  { value: "guardian", label: "Vormund" },
  { value: "relative", label: "Verwandte/r" },
  { value: "other", label: "Sonstige" },
] as const;

// Contact method options
export const CONTACT_METHODS = [
  { value: "email", label: "E-Mail" },
  { value: "phone", label: "Telefon" },
  { value: "mobile", label: "Mobiltelefon" },
] as const;

// Language preference options
export const LANGUAGE_PREFERENCES = [
  { value: "de", label: "Deutsch" },
  { value: "en", label: "English" },
  { value: "tr", label: "Türkisch" },
  { value: "ar", label: "Arabisch" },
  { value: "ru", label: "Russisch" },
  { value: "pl", label: "Polnisch" },
  { value: "uk", label: "Ukrainisch" },
  { value: "fa", label: "Persisch" },
  { value: "ro", label: "Rumänisch" },
  { value: "other", label: "Sonstige" },
] as const;

// Helper to get language label from code
export function getLanguageLabel(code: string): string {
  const lang = LANGUAGE_PREFERENCES.find((l) => l.value === code);
  return lang?.label ?? code.toUpperCase();
}

// Helper to get full name
export function getGuardianFullName(guardian: Guardian): string {
  return `${guardian.firstName} ${guardian.lastName}`;
}

// Helper to get primary contact
export function getGuardianPrimaryContact(guardian: Guardian): string {
  if (guardian.preferredContactMethod === "email" && guardian.email) {
    return guardian.email;
  }

  // Find phone by preferred contact method
  if (
    guardian.preferredContactMethod === "mobile" ||
    guardian.preferredContactMethod === "sms"
  ) {
    const mobilePhone = guardian.phoneNumbers.find(
      (p) => p.phoneType === "mobile",
    );
    if (mobilePhone) return mobilePhone.phoneNumber;
  }

  if (guardian.preferredContactMethod === "phone") {
    const homePhone = guardian.phoneNumbers.find((p) => p.phoneType === "home");
    if (homePhone) return homePhone.phoneNumber;
  }

  // Fallback: return any available contact
  if (guardian.email) return guardian.email;

  // Return primary phone or first phone
  const primaryPhone = guardian.phoneNumbers.find((p) => p.isPrimary);
  if (primaryPhone) return primaryPhone.phoneNumber;

  const firstPhone = guardian.phoneNumbers[0];
  if (firstPhone) return firstPhone.phoneNumber;

  return "Keine Kontaktdaten";
}

// Helper to get relationship type label
export function getRelationshipTypeLabel(type: string): string {
  const found = RELATIONSHIP_TYPES.find((t) => t.value === type);
  return found ? found.label : type;
}

// Phone number request types
export interface PhoneNumberCreateRequest {
  phoneNumber: string;
  phoneType: PhoneType;
  label?: string;
  isPrimary?: boolean;
}

export interface PhoneNumberUpdateRequest {
  phoneNumber?: string;
  phoneType?: PhoneType;
  label?: string;
}

interface BackendPhoneNumberCreateRequest {
  phone_number: string;
  phone_type: PhoneType;
  label?: string;
  is_primary?: boolean;
}

interface BackendPhoneNumberUpdateRequest {
  phone_number?: string;
  phone_type?: PhoneType;
  label?: string;
}

// Map phone number create request to backend format
export function mapPhoneNumberCreateToBackend(
  data: PhoneNumberCreateRequest,
): BackendPhoneNumberCreateRequest {
  return {
    phone_number: data.phoneNumber,
    phone_type: data.phoneType,
    label: data.label,
    is_primary: data.isPrimary,
  };
}

// Map phone number update request to backend format
export function mapPhoneNumberUpdateToBackend(
  data: PhoneNumberUpdateRequest,
): BackendPhoneNumberUpdateRequest {
  const result: BackendPhoneNumberUpdateRequest = {};
  if (data.phoneNumber !== undefined) result.phone_number = data.phoneNumber;
  if (data.phoneType !== undefined) result.phone_type = data.phoneType;
  if (data.label !== undefined) result.label = data.label;
  return result;
}

// Helper to get phone type label
export function getPhoneTypeLabel(type: PhoneType): string {
  return PHONE_TYPE_LABELS[type] ?? type;
}

// Helper to get primary phone number
export function getPrimaryPhoneNumber(
  guardian: Guardian,
): PhoneNumber | undefined {
  return guardian.phoneNumbers.find((p) => p.isPrimary);
}

// Helper to format phone number display (includes type and label)
export function formatPhoneNumberDisplay(phone: PhoneNumber): string {
  const typeLabel = getPhoneTypeLabel(phone.phoneType);
  if (phone.label && phone.label !== typeLabel) {
    return `${phone.phoneNumber} (${typeLabel} - ${phone.label})`;
  }
  return `${phone.phoneNumber} (${typeLabel})`;
}
