import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";
import { within } from "@testing-library/react";
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
const { list, activities, create } = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  activities: vi.fn(),
}));
vi.mock("~/lib/student-notes-api", async (original) => ({
  ...(await original<typeof import("~/lib/student-notes-api")>()),
  studentNotesService: { list, listActivities: activities, create },
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
  it.each([
    ["journal", 503],
    ["permanent", 503],
    ["journal", 403],
  ] as const)(
    "reports a failed %s read after saving (%s)",
    async (kind, status) => {
      list.mockReset().mockResolvedValue([note]);
      activities.mockReset().mockResolvedValue([]);
      create.mockReset().mockResolvedValue(undefined);
      const view = render(
        <SWRConfig
          value={{ provider: () => new Map(), shouldRetryOnError: false }}
        >
          <ToastProvider>
            <SWRScopeBoundary />
            <StudentNotizenTab studentId="7" />
          </ToastProvider>
        </SWRConfig>,
      );
      expect(await screen.findByText(note.body)).toBeVisible();
      fireEvent.click(screen.getByRole("button", { name: "Neue Notiz" }));
      if (kind === "permanent")
        fireEvent.click(
          screen.getByRole("button", { name: "Dauerhafter Hinweis" }),
        );
      fireEvent.change(screen.getByLabelText("Notiz"), {
        target: { value: "Gespeicherte Notiz" },
      });
      list.mockImplementation((_id: string, filter?: string) =>
        kind === "journal" || filter === "permanent"
          ? Promise.reject(new ApiError("failed", status))
          : Promise.resolve([note]),
      );
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
      const code =
        status === 403 ? "general.permission" : "general.unavailable";
      // Anker: ein erfolgreicher Schreibaufruf mit gescheitertem GET nennt den gespeicherten Stand.
      const alertText = `${catalogText(code, "die Anzeige der Notizen")} ${SAVED_BUT_STALE_SUFFIX}`;
      expect(await screen.findByText(alertText)).toBeVisible();
      if (status === 403) {
        expect(screen.queryByText(note.body)).not.toBeInTheDocument();
        expect(
          screen.queryByRole("button", { name: "Neue Notiz" }),
        ).not.toBeInTheDocument();
      } else {
        expect(screen.getByText(note.body)).toBeVisible();
        list.mockResolvedValue([{ ...note, body: "Gespeicherte Notiz" }]);
        const alert = screen
          .getByText(alertText)
          .closest<HTMLElement>('[role="alert"]')!;
        fireEvent.click(
          within(alert).getByRole("button", { name: "Wiederholen" }),
        );
        expect(await screen.findByText("Gespeicherte Notiz")).toBeVisible();
        expect(screen.queryByText(alertText)).not.toBeInTheDocument();
      }
      expect(create).toHaveBeenCalledTimes(1);
      view.unmount();
    },
  );

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
