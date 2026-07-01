import type { Meta, StoryObj, Decorator } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
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

// The component fetches its own data (via staffAbsenceService -> sessionFetch
// -> global fetch). Stub `fetch` locally per story so it has data to render,
// without touching the global preview decorators.
function withMockedAbsences(
  absences: readonly StaffAbsenceRow[],
  quotaOverride: StaffVacationQuotaSummary | null = quota,
): Decorator {
  return (Story) => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      if (url.includes("/vacation/quota")) {
        if (quotaOverride === null) {
          return Promise.resolve(new Response(null, { status: 404 }));
        }
        return Promise.resolve(
          new Response(JSON.stringify({ data: quotaOverride }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      if (url.includes("/absences")) {
        return Promise.resolve(
          new Response(JSON.stringify({ data: absences }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      return originalFetch(input, init);
    }) as typeof fetch;

    return (
      <ToastProvider>
        <Story />
      </ToastProvider>
    );
  };
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
  },
  decorators: [
    withMockedAbsences([pendingAbsence, upcomingAbsence, historyAbsence]),
  ],
};

export const ReadOnly: Story = {
  args: {
    staffId: "1",
    canEdit: false,
  },
  decorators: [
    withMockedAbsences([pendingAbsence, upcomingAbsence, historyAbsence]),
  ],
};

export const Empty: Story = {
  args: {
    staffId: "1",
    canEdit: true,
  },
  decorators: [withMockedAbsences([])],
};
