"use client";

import {
  GUARDIAN_ROLE_OPTIONS,
  RELATIONSHIP_TYPES,
  type GuardianRole,
} from "@/lib/guardian-helpers";
import { CustomSelect } from "~/components/ui/custom-select";
import { ParentVisibleBadge } from "~/components/ui/parent-visible-badge";
import { PARENT_VISIBLE_HINTS } from "~/lib/parent-visible-fields";

// Shared relationship UI used by BOTH the multi-guardian form (create/edit) and
// the existing-guardian picker (#1513). Extracting these blocks keeps the two
// paths from drifting on relationship-type options, permission labels, or the
// emergency-contact styling. The markup is byte-for-byte the original
// GuardianFormModal markup, so the form renders identically after the swap.

interface RelationshipTypeSelectProps {
  readonly id: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
}

// RelationshipTypeSelect renders the "Beziehung zum Kind" dropdown.
export function RelationshipTypeSelect({
  id,
  value,
  onChange,
  disabled = false,
}: RelationshipTypeSelectProps) {
  return (
    <div>
      <label
        id={`${id}-label`}
        htmlFor={id}
        className="mb-1 block text-xs font-medium text-gray-700"
      >
        Beziehung zum Kind
      </label>
      <CustomSelect
        id={id}
        ariaLabelledBy={`${id}-label`}
        value={value}
        options={RELATIONSHIP_TYPES}
        onChange={onChange}
        disabled={disabled}
      />
    </div>
  );
}

interface GuardianRoleSelectProps {
  readonly id: string;
  readonly value: GuardianRole;
  readonly onChange: (value: GuardianRole) => void;
  readonly disabled?: boolean;
}

export function GuardianRoleSelect({
  id,
  value,
  onChange,
  disabled = false,
}: GuardianRoleSelectProps) {
  return (
    <div>
      <label
        id={`${id}-label`}
        htmlFor={id}
        className="mb-1 block text-xs font-medium text-gray-700"
      >
        Portalrolle
      </label>
      <CustomSelect
        id={id}
        ariaLabelledBy={`${id}-label`}
        value={value}
        options={GUARDIAN_ROLE_OPTIONS}
        onChange={(next) => onChange(next as GuardianRole)}
        disabled={disabled}
      />
    </div>
  );
}

export function guardianRoleOperationalDefaults(
  role: GuardianRole,
): Partial<Record<RelationshipFlag, boolean>> {
  switch (role) {
    case "primary_guardian":
      return { isPrimary: true, canPickup: false, isEmergencyContact: false };
    case "legal_guardian":
    case "co_guardian":
      return { isPrimary: false, canPickup: false, isEmergencyContact: false };
    case "emergency_contact":
      return { isPrimary: false, canPickup: false, isEmergencyContact: true };
    case "pickup_only":
      return { isPrimary: false, canPickup: true, isEmergencyContact: false };
    case "social_worker":
      return { isPrimary: false, canPickup: false, isEmergencyContact: false };
    case "custom":
      return {};
  }
}

export type RelationshipFlag = "isPrimary" | "canPickup" | "isEmergencyContact";

export function defaultGuardianRoleForRelationshipType(
  relationshipType: string,
): GuardianRole {
  switch (relationshipType.trim().toLowerCase()) {
    case "parent":
    case "guardian":
      return "legal_guardian";
    default:
      return "custom";
  }
}

interface RelationshipPermissionsFieldsProps {
  readonly isPrimary: boolean;
  readonly canPickup: boolean;
  readonly isEmergencyContact: boolean;
  readonly onChange: (field: RelationshipFlag, value: boolean) => void;
  readonly disabled?: boolean;
}

// RelationshipPermissionsFields renders the "Berechtigungen" block
// (Hauptansprechpartner, Abholberechtigt) plus the "Notfallkontakt" block.
export function RelationshipPermissionsFields({
  isPrimary,
  canPickup,
  isEmergencyContact,
  onChange,
  disabled = false,
}: RelationshipPermissionsFieldsProps) {
  return (
    <>
      {/* Relationship Flags */}
      <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
        <h3 className="mb-3 flex flex-wrap items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
          <svg
            className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
            />
          </svg>
          Berechtigungen
          <ParentVisibleBadge hint={PARENT_VISIBLE_HINTS.guardianPermissions} />
        </h3>
        <div className="space-y-2">
          <label className="flex cursor-pointer items-center gap-3 rounded-lg px-2 py-1.5 transition-colors hover:bg-white/50">
            <input
              type="checkbox"
              checked={isPrimary}
              onChange={(e) => onChange("isPrimary", e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-purple-600 focus:ring-purple-600"
              disabled={disabled}
            />
            <span className="text-sm text-gray-700">Hauptansprechpartner</span>
          </label>
          <label className="flex cursor-pointer items-center gap-3 rounded-lg px-2 py-1.5 transition-colors hover:bg-white/50">
            <input
              type="checkbox"
              checked={canPickup}
              onChange={(e) => onChange("canPickup", e.target.checked)}
              className="h-4 w-4 rounded border-gray-300 text-green-600 focus:ring-green-600"
              disabled={disabled}
            />
            <span className="text-sm text-gray-700">Abholberechtigt</span>
          </label>
        </div>
      </div>

      {/* Emergency Contact */}
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-2">
        <label className="group flex cursor-pointer items-center gap-3">
          <input
            type="checkbox"
            checked={isEmergencyContact}
            onChange={(e) => onChange("isEmergencyContact", e.target.checked)}
            className="h-4 w-4 rounded border-gray-300 text-red-600 focus:ring-red-600"
            disabled={disabled}
            aria-label="Als Notfallkontakt markieren"
          />
          <div className="flex items-center gap-2">
            <svg
              className="h-5 w-5 text-red-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            <span className="text-sm font-medium text-red-900">
              Notfallkontakt
            </span>
          </div>
        </label>
        <ParentVisibleBadge hint={PARENT_VISIBLE_HINTS.guardianPermissions} />
      </div>
    </>
  );
}
