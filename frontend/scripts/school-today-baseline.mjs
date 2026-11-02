// Reviewed browser-local inputs and instants, not school-day derivations.
// Shrink-only: moving a line may update its fingerprint, not add exemptions.
export default {
  // Only the clock-source sites in the central helpers, not the whole file.
  "src/lib/date-helpers.ts": [
    "todayISO: return toISODate(new Date());",
    "berlinTodayISO: export function berlinTodayISO(at: Date = new Date()): string {",
    "berlinDayStart: export function berlinDayStart(at: Date = new Date()): Date {",
    "getStartDateForTimeRange: referenceDate: Date = new Date(),",
    "relativeDaysLabel: at: Date = new Date(),",
  ],
  "src/lib/activity-api.ts": [
    "created_at: new Date(),",
    "updated_at: new Date(),",
  ],
  "src/lib/pickup-helpers.ts": [
    "const [now, setNow] = useState(() => new Date());",
    "setNow(new Date());",
    "const resync = () => setNow(new Date());",
  ],
  "src/lib/dashboard-helpers.ts": ["const now = new Date();"],
  "src/lib/survey-editor.ts": ["now: Date = new Date(),"],
  "src/lib/logger.ts": [
    "timestamp: new Date().toISOString(),",
    "timestamp: new Date().toISOString(),",
    "timestamp: new Date().toISOString(),",
  ],
  "src/lib/timetable-helpers.ts": ["now: Date = new Date(),"],
  "src/lib/time-tracking-helpers.ts": ["now: Date = new Date(),"],
  "src/lib/greeting.ts": [
    "export function getTimeBasedGreeting(now: Date = new Date()): string {",
  ],
  "src/lib/client-log-ingest.server.ts": [
    "api_timestamp: new Date().toISOString(),",
  ],
  "src/lib/activity-helpers.ts": [": new Date(),", ": new Date(),"],
  "src/lib/hooks/use-reminders.ts": ["const now = new Date();"],
  "src/components/ui/date-picker.tsx": [
    "const thisYear = new Date().getFullYear();",
  ],
  "src/components/home/home-card-rows.tsx": ["}).format(new Date());"],
  "src/components/home/now-strip.tsx": [
    "? startableOwnBlock(day.data ?? [], new Date())",
  ],
  "src/components/home/my-day-block.tsx": [
    'block.status === "planned" && canStartPlannedInstance(block, new Date());',
  ],
  "src/components/enrollment/date-of-birth-picker.tsx": [
    "const currentYear = new Date().getFullYear();",
  ],
  "src/components/enrollment/admin-child-data-correction.tsx": [
    "max={todayISO()}",
  ],
  "src/components/class-day/class-day-class.tsx": [
    "const at = now ?? new Date();",
  ],
  "src/components/class-day/day-changes.ts": [
    "now: Date = new Date(),",
    "now: Date = new Date(),",
  ],
  "src/components/announcements/announcement-reminder-dialog.tsx": [
    "now: Date = new Date(),",
  ],
  "src/components/announcements/announcement-detail.tsx": [
    "new Date(announcement.response_deadline) <= new Date();",
  ],
  "src/components/announcements/announcement-meta.tsx": [
    "now: Date = new Date(),",
    "now: Date = new Date(),",
  ],
  "src/components/students/student-form-fields.tsx": ["max={todayISO()}"],
  "src/components/students/personal-info-edit-panel.tsx": ["max={todayISO()}"],
  "src/components/students/student-card-clock.tsx": [
    "return explicitNow ?? listClock ?? new Date();",
  ],
  "src/components/students/request-review-card.tsx": [
    "setNow(new Date());",
    "const timer = window.setInterval(() => setNow(new Date()), 60_000);",
  ],
  "src/components/students/early-checkout-note.tsx": ["now: new Date(),"],
  "src/components/parent/child-master-data-request-modal.tsx": [
    "max={todayISO()}",
  ],
  "src/components/surveys/survey-questionnaire.tsx": [
    "const closed = new Date(form.survey.closes_at) <= new Date();",
  ],
  "src/components/display/display-clock.tsx": [
    "setNow(new Date());",
    "const timer = setInterval(() => setNow(new Date()), 1000);",
  ],
  "src/components/display/display-dashboard-view.tsx": [
    "const now = new Date();",
  ],
  "src/components/staff/uebersicht-tab.tsx": ["new Date(),"],
  "src/components/staff/staff-session-table.tsx": ["new Date(),"],
  "src/components/timetable/tagesplan-view.tsx": [
    "canStartPlannedInstance(instance, new Date());",
  ],
  "src/components/parent/start/parent-start-page.tsx": [
    "now: Date = new Date(),",
    "function berlinHour(now: Date = new Date()): number {",
  ],
  "src/components/parent/news/survey-section.tsx": [
    "!item.survey.participated && new Date(item.survey.closes_at) > new Date()",
  ],
  "src/components/parent/news/news-components.tsx": [
    "new Date(item.response_deadline) <= new Date()",
    "const closed = new Date(survey.closes_at) <= new Date();",
  ],
  "src/app/api/health/route.ts": ["timestamp: new Date().toISOString(),"],
  "src/app/api/activities/route.ts": [
    "created_at: new Date(),",
    "updated_at: new Date(),",
  ],
  "src/app/api/activities/quick-create/route.ts": [
    "created_at: new Date().toISOString(),",
  ],
  "src/app/[tenant]/(protected)/time-tracking/page.tsx": [
    "const now = new Date();",
    "const now = new Date();",
    "const tzOffset = -new Date().getTimezoneOffset();",
  ],
};
