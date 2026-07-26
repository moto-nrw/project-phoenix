import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { AbwesenheitenTab } from "./abwesenheiten-tab";
import type {
  StaffAbsenceRow,
  StaffVacationQuotaSummary,
} from "~/lib/staff-api";

const quota: StaffVacationQuotaSummary = {
  staff_id: 1,
  year: new Date().getFullYear(),
  entitled_days: 30,
  carryover_days: 2,
  taken_days: 8,
  reserved_days: 3,
  remaining_days: 21,
};

const pendingAbsence: StaffAbsenceRow = {
  id: 1,
  staff_id: 1,
  absence_type: "vacation",
  date_start: "2026-08-10",
  date_end: "2026-08-12",
  half_day: false,
  note: "Familienurlaub",
  status: "requested",
  working_days: 3,
  requested_at: "2026-07-01T09:00:00Z",
};

const upcomingAbsence: StaffAbsenceRow = {
  id: 2,
  staff_id: 1,
  absence_type: "training",
  date_start: "2026-09-01",
  date_end: "2026-09-01",
  half_day: false,
  note: "",
  status: "approved",
  working_days: 1,
};

const historyAbsence: StaffAbsenceRow = {
  id: 3,
  staff_id: 1,
  absence_type: "sick",
  date_start: "2026-05-02",
  date_end: "2026-05-03",
  half_day: false,
  note: "",
  status: "declined",
  decision_note: "Bereits anderweitig vertreten",
  working_days: 2,
};

function withMockedAbsences(
  absences: readonly StaffAbsenceRow[],
  quotaOverride: StaffVacationQuotaSummary | null = quota,
) {
  return installStorybookFetch(({ url }) => {
    if (url.includes("/vacation/quota")) {
      if (quotaOverride === null) {
        return new Response(null, { status: 404 });
      }
      return jsonResponse({ data: quotaOverride });
    }

    if (url.includes("/absences")) {
      return jsonResponse({ data: absences });
    }

    return undefined;
  });
}

const meta = {
  title: "staff/AbwesenheitenTab",
  component: AbwesenheitenTab,
} satisfies Meta<typeof AbwesenheitenTab>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithPendingAndHistory: Story = {
  args: {
    staffId: "1",
    canEdit: true,
    canManageSickReports: true,
  },
  beforeEach: () =>
    withMockedAbsences([pendingAbsence, upcomingAbsence, historyAbsence]),
};

export const ReadOnly: Story = {
  args: {
    staffId: "1",
    canEdit: false,
    canManageSickReports: false,
  },
  beforeEach: () =>
    withMockedAbsences([pendingAbsence, upcomingAbsence, historyAbsence]),
};

export const Empty: Story = {
  args: {
    staffId: "1",
    canEdit: true,
    canManageSickReports: true,
  },
  beforeEach: () => withMockedAbsences([]),
};
