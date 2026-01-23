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
  occupation?: string;
  employer?: string;
  notes?: string;
  hasAccount: boolean;
  accountId?: string;
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
  occupation?: string;
  employer?: string;
  notes?: string;
  has_account: boolean;
  account_id?: number;
}

// Student-Guardian Relationship
export interface StudentGuardianRelationship {
  guardianId: string;
  relationshipId: string;
  relationshipType: string;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  pickupNotes?: string;
  emergencyPriority: number;
}

// Backend Student-Guardian Relationship
export interface BackendStudentGuardianRelationship {
  guardian_id: number;
  relationship_id: number;
  relationship_type: string;
  is_primary: boolean;
  is_emergency_contact: boolean;
  can_pickup: boolean;
  pickup_notes?: string;
  emergency_priority: number;
}

// Guardian with Relationship (for student detail view)
export interface GuardianWithRelationship extends Guardian {
  relationshipId: string;
  relationshipType: string;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  pickupNotes?: string;
  emergencyPriority: number;
}

// Backend Guardian with Relationship
export interface BackendGuardianWithRelationship {
  guardian: BackendGuardianProfile;
  relationship_id: number;
  relationship_type: string;
  is_primary: boolean;
  is_emergency_contact: boolean;
  can_pickup: boolean;
  pickup_notes?: string;
  emergency_priority: number;
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
  occupation?: string;
  employer?: string;
  notes?: string;
}

// Backend Guardian Create Request
export interface BackendGuardianCreateRequest {
  first_name: string;
  last_name: string;
  email?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  preferred_contact_method: string;
  language_preference: string;
  occupation?: string;
  employer?: string;
  notes?: string;
}

// Student-Guardian Link Request
export interface StudentGuardianLinkRequest {
  guardianProfileId: string;
  relationshipType: string;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  pickupNotes?: string;
  emergencyPriority: number;
}

// Backend Student-Guardian Link Request
export interface BackendStudentGuardianLinkRequest {
  guardian_profile_id: number;
  relationship_type: string;
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
    occupation: data.occupation,
    employer: data.employer,
    notes: data.notes,
    hasAccount: data.has_account,
    accountId: data.account_id?.toString(),
  };
}

export function mapGuardianWithRelationshipResponse(
  data: BackendGuardianWithRelationship,
): GuardianWithRelationship {
  return {
    ...mapGuardianResponse(data.guardian),
    relationshipId: data.relationship_id.toString(),
    relationshipType: data.relationship_type,
    isPrimary: data.is_primary,
    isEmergencyContact: data.is_emergency_contact,
    canPickup: data.can_pickup,
    pickupNotes: data.pickup_notes,
    emergencyPriority: data.emergency_priority,
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
    preferred_contact_method: data.preferredContactMethod ?? "email",
    language_preference: data.languagePreference ?? "de",
    occupation: data.occupation,
    employer: data.employer,
    notes: data.notes,
  };
}

export function mapStudentGuardianLinkToBackend(
  data: StudentGuardianLinkRequest,
): BackendStudentGuardianLinkRequest {
  return {
    guardian_profile_id: Number.parseInt(data.guardianProfileId),
    relationship_type: data.relationshipType,
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
  { value: "tr", label: "Türkçe" },
  { value: "ar", label: "العربية" },
  { value: "other", label: "Sonstige" },
] as const;

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

export interface BackendPhoneNumberCreateRequest {
  phone_number: string;
  phone_type: PhoneType;
  label?: string;
  is_primary?: boolean;
}

export interface BackendPhoneNumberUpdateRequest {
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
