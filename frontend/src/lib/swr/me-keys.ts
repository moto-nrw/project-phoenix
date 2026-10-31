import { SWR_FAMILIES } from "./shell-keys";
export const meKeys = {
  family: SWR_FAMILIES.me,
  key: () => "me-staff",
} as const;
