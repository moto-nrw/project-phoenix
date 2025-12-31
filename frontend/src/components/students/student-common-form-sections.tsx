/**
 * Common form sections shared between student create and edit modals
 * Eliminates JSX duplication while maintaining consistent UI
 */

import type { Student } from "@/lib/api";
import {
  HealthInfoSection,
  SupervisorNotesSection,
  AdditionalInfoSection,
  PrivacyConsentSection,
} from "./student-form-fields";

interface StudentCommonFormSectionsProps {
  readonly formData: Partial<Student>;
  readonly errors: Record<string, string>;
  readonly onChange: (
    field: keyof Student,
    value: string | boolean | number | null,
  ) => void;
}

/**
 * Renders common form sections for student forms
 * Includes: Health Info, Supervisor Notes, Additional Info, Privacy Consent
 */
export function StudentCommonFormSections({
  formData,
  errors,
  onChange,
}: Readonly<StudentCommonFormSectionsProps>) {
  return (
    <>
      {/* Health Information */}
      <HealthInfoSection
        value={formData.health_info}
        onChange={(v) => onChange("health_info", v)}
      />

      {/* Supervisor Notes */}
      <SupervisorNotesSection
        value={formData.supervisor_notes}
        onChange={(v) => onChange("supervisor_notes", v)}
      />

      {/* Additional Information */}
      <AdditionalInfoSection
        value={formData.extra_info}
        onChange={(v) => onChange("extra_info", v)}
      />

      {/* Privacy & Data Retention */}
      <PrivacyConsentSection
        formData={formData}
        onChange={onChange}
        errors={errors}
      />
    </>
  );
}
