// Backend response types
export interface BackendGuardian {
  id: number;
  account_id?: number;
  first_name: string;
  last_name: string;
  phone: string;
  phone_secondary?: string;
  email: string;
  address?: string;
  city?: string;
  postal_code?: string;
  country: string;
  is_emergency_contact: boolean;
  emergency_priority?: number;
  notes?: string;
  language_preference: string;
  notification_preferences?: Record<string, unknown>;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface BackendStudentGuardian {
  id: number;
  student_id: number;
  guardian_id: number;
  relationship_type: string;
  is_primary: boolean;
  is_emergency_contact: boolean;
  can_pickup: boolean;
  permissions?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  guardian?: BackendGuardian;
}

// Frontend types
export interface Guardian {
  id: string;
  accountId?: string;
  firstName: string;
  lastName: string;
  phone: string;
  phoneSecondary?: string;
  email: string;
  address?: string;
  city?: string;
  postalCode?: string;
  country: string;
  isEmergencyContact: boolean;
  emergencyPriority?: number;
  notes?: string;
  languagePreference: string;
  notificationPreferences?: Record<string, unknown>;
  active: boolean;
  createdAt: Date;
  updatedAt: Date;
}

export interface StudentGuardian {
  id: string;
  studentId: string;
  guardianId: string;
  relationshipType: string;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  permissions?: Record<string, unknown>;
  createdAt: Date;
  updatedAt: Date;
  guardian?: Guardian;
}

// Relationship type options
export const RELATIONSHIP_TYPES = [
  { value: "mother", label: "Mutter" },
  { value: "father", label: "Vater" },
  { value: "parent", label: "Elternteil" },
  { value: "guardian", label: "Erziehungsberechtigte/r" },
  { value: "grandparent", label: "Großelternteil" },
  { value: "relative", label: "Verwandte/r" },
  { value: "other", label: "Andere/r" },
] as const;

// Mapping functions
export function mapGuardianResponse(data: BackendGuardian): Guardian {
  return {
    id: data.id.toString(),
    accountId: data.account_id?.toString(),
    firstName: data.first_name,
    lastName: data.last_name,
    phone: data.phone,
    phoneSecondary: data.phone_secondary,
    email: data.email,
    address: data.address,
    city: data.city,
    postalCode: data.postal_code,
    country: data.country,
    isEmergencyContact: data.is_emergency_contact,
    emergencyPriority: data.emergency_priority,
    notes: data.notes,
    languagePreference: data.language_preference,
    notificationPreferences: data.notification_preferences,
    active: data.active,
    createdAt: new Date(data.created_at),
    updatedAt: new Date(data.updated_at),
  };
}

export function mapStudentGuardianResponse(data: BackendStudentGuardian): StudentGuardian {
  return {
    id: data.id.toString(),
    studentId: data.student_id.toString(),
    guardianId: data.guardian_id.toString(),
    relationshipType: data.relationship_type,
    isPrimary: data.is_primary,
    isEmergencyContact: data.is_emergency_contact,
    canPickup: data.can_pickup,
    permissions: data.permissions,
    createdAt: new Date(data.created_at),
    updatedAt: new Date(data.updated_at),
    guardian: data.guardian ? mapGuardianResponse(data.guardian) : undefined,
  };
}

// Request mapping (frontend → backend)
export function mapGuardianRequest(guardian: Partial<Guardian>): Partial<BackendGuardian> {
  return {
    first_name: guardian.firstName,
    last_name: guardian.lastName,
    phone: guardian.phone,
    phone_secondary: guardian.phoneSecondary,
    email: guardian.email,
    address: guardian.address,
    city: guardian.city,
    postal_code: guardian.postalCode,
    country: guardian.country,
    is_emergency_contact: guardian.isEmergencyContact,
    emergency_priority: guardian.emergencyPriority,
    notes: guardian.notes,
    language_preference: guardian.languagePreference,
    notification_preferences: guardian.notificationPreferences,
    active: guardian.active,
  };
}

export function mapStudentGuardianRequest(
  relationship: Partial<StudentGuardian>
): Partial<BackendStudentGuardian> {
  return {
    student_id: relationship.studentId ? parseInt(relationship.studentId) : undefined,
    guardian_id: relationship.guardianId ? parseInt(relationship.guardianId) : undefined,
    relationship_type: relationship.relationshipType,
    is_primary: relationship.isPrimary,
    is_emergency_contact: relationship.isEmergencyContact,
    can_pickup: relationship.canPickup,
    permissions: relationship.permissions,
  };
}

// Helper functions
export function getGuardianFullName(guardian: Guardian): string {
  return `${guardian.firstName} ${guardian.lastName}`.trim();
}

export function getRelationshipTypeLabel(type: string): string {
  const found = RELATIONSHIP_TYPES.find((r) => r.value === type);
  return found?.label ?? type;
}

export function formatGuardianAddress(guardian: Guardian): string | null {
  const parts = [guardian.address, guardian.postalCode, guardian.city].filter(Boolean);
  return parts.length > 0 ? parts.join(", ") : null;
}
