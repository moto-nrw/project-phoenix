import type { StatusBadgeTone } from "~/components/ui/status-badge";
import type {
  HolidayPlacement,
  HolidayRegistrationEntryStatus,
} from "~/lib/parent-holiday-registration-entries";

export const HOLIDAY_PLACEMENT_BADGES = {
  placed: { key: "placementPlaced", tone: "green" },
  waitlisted: { key: "placementWaitlisted", tone: "warning" },
  pending: { key: "placementPending", tone: "purple" },
} as const satisfies Record<
  HolidayPlacement,
  { key: string; tone: StatusBadgeTone }
>;

export const HOLIDAY_REGISTRATION_STATUS_TONES = {
  received: "blue",
  confirmed: "green",
  cancelled: "gray",
} as const satisfies Record<HolidayRegistrationEntryStatus, StatusBadgeTone>;
