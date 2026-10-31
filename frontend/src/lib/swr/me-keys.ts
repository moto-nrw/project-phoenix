import { SWR_FAMILIES } from "./families";
export const meKeys = {
  family: SWR_FAMILIES.me,
  key: () => "me-staff",
} as const;
