import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import type { StudentNote } from "~/lib/student-notes-api";
import { useInvalidateMatching } from "~/lib/swr";
import { studentKeys } from "~/lib/swr/portal-keys";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { ToastProvider } from "~/contexts/ToastContext";
import { StudentNotizenTab } from "./student-notizen-tab";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", tenantId: 1 }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));
const { list, activities } = vi.hoisted(() => ({
  list: vi.fn(),
  activities: vi.fn(),
}));
vi.mock("~/lib/student-notes-api", async (original) => ({
  ...(await original<typeof import("~/lib/student-notes-api")>()),
  studentNotesService: { list, listActivities: activities },
}));

const note: StudentNote = {
  id: "1",
  studentId: "7",
  kind: "journal",
  visibility: "all_staff",
  category: "",
  body: "Vertrauliche Notiz",
  origin: "staff",
  authorName: "Sara",
  subjectDate: "2026-09-09",
  activityGroupId: "",
  educationGroupId: "",
  sharedWithSchool: false,
  canEdit: true,
  canDelete: true,
  edited: false,
  createdAt: "2026-09-09T10:00:00Z",
  updatedAt: "2026-09-09T10:00:00Z",
};

describe("student note access", () => {
  it.each([403, 503])(
    "handles a later %s without retaining refused notes",
    async (status) => {
      list.mockReset().mockResolvedValue([note]);
      activities.mockReset().mockResolvedValue([]);
      const cache = new Map();
      let refresh!: ReturnType<typeof useInvalidateMatching>;
      function Refresh() {
        refresh = useInvalidateMatching();
        return null;
      }
      const view = render(
        <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
          <ToastProvider>
            <SWRScopeBoundary />
            <Refresh />
            <StudentNotizenTab studentId="7" />
          </ToastProvider>
        </SWRConfig>,
      );
      // Anker: die spätere verweigerte Notiz und ihre Aktionen sind geladen.
      expect(await screen.findByText(note.body)).toBeVisible();
      fireEvent.click(screen.getByRole("button", { name: /Aktionen/ }));
      expect(
        await screen.findByRole("menuitem", { name: /Bearbeiten/ }),
      ).toBeVisible();
      list.mockRejectedValue(new ApiError("failed", status));
      await act(async () => {
        await refresh((key) => key === studentKeys.notes("7"));
      });
      expect(await screen.findByRole("alert")).toBeVisible();
      if (status === 403) {
        await waitFor(() =>
          expect(screen.queryByText(note.body)).not.toBeInTheDocument(),
        );
        expect(
          screen.queryByRole("button", { name: "Neue Notiz" }),
        ).not.toBeInTheDocument();
        const cached = [...cache.entries()].find(([key]) =>
          key.endsWith(`:${studentKeys.notes("7")}`),
        )?.[1];
        expect(cached?.data).toBeUndefined();
      } else {
        expect(screen.getByText(note.body)).toBeVisible();
        expect(
          screen.getByRole("button", { name: "Neue Notiz" }),
        ).toBeVisible();
      }
      view.unmount();
    },
  );
});
