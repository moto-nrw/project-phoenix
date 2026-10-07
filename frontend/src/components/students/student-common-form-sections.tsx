/**
 * Common form sections shared between student create and edit modals
 * Eliminates JSX duplication while maintaining consistent UI
 */

import type { Student } from "@/lib/api";
import type { BusDays } from "~/lib/student-helpers";
import {
  HealthInfoSection,
  AdditionalInfoSection,
  PrivacyConsentSection,
} from "./student-form-fields";

interface StudentCommonFormSectionsProps {
  readonly formData: Partial<Student>;
  /** Hint per API field name, from the form's shared error path (#2513). */
  readonly fieldError: (name: string) => string | undefined;
  readonly onChange: (
    field: keyof Student,
    value: string | boolean | number | BusDays | null,
  ) => void;
}

/**
 * Renders common form sections for student forms
 * Includes: Health Info, Supervisor Notes, Additional Info, Privacy Consent
 */
export function StudentCommonFormSections({
  formData,
  fieldError,
  onChange,
}: Readonly<StudentCommonFormSectionsProps>) {
  return (
    <>
      {/* Health Information */}
      <HealthInfoSection
        value={formData.health_info}
        onChange={(v) => onChange("health_info", v)}
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
        fieldError={fieldError}
      />
    </>
  );
}
