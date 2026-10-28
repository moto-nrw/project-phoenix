import { userContextService } from "~/lib/usercontext-api";
import { meKeys } from "./me-keys";
export const me = {
  ...meKeys,
  fetcher: () => userContextService.getCurrentStaff(),
} as const;
