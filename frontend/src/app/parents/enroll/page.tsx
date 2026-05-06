import { EnrollSchoolPicker } from "~/components/parent/enroll-school-picker";

/**
 * School picker for the parent enrollment flow. Lists every (school,
 * open phase) pair the parent could submit a new enrollment to. Data
 * source: GET /api/parent/me/enrollable-schools → backend cross-tenant
 * aggregation. Each phase links to /parents/enroll/{slug}/{phaseId}
 * which (PR 11 commit 2) embeds the public enrollment form.
 */
export default function ParentEnrollPickerPage() {
  return <EnrollSchoolPicker />;
}
