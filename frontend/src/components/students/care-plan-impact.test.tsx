import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type { RosterImpact } from "~/lib/roster-impact";
import { CarePlanImpact } from "./care-plan-impact";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  removeStudentTemplateDays: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/lib/roster-impact", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/roster-impact")>()),
  removeStudentTemplateDays: mocks.removeStudentTemplateDays,
}));

const impact: RosterImpact = {
  effectiveFrom: "2027-05-31",
  adjusted: [
    {
      templateId: "7",
      name: "Kurzbetreuung",
      removedWeekdays: [5],
      addedWeekdays: [],
    },
  ],
  notAdjusted: [
    { templateId: "9", name: "RDST 1b", weekdays: [5], viaTarget: true },
  ],
};

function renderImpact(
  value: RosterImpact = impact,
  studentId: string | undefined = "42",
) {
  return render(
    <ToastProvider>
      <CarePlanImpact impact={value} studentId={studentId} />
    </ToastProvider>,
  );
}

function grantPlanEditing(granted: boolean) {
  mocks.useSession.mockReturnValue({
    data: {
      user: {
        id: "1",
        token: "t",
        permissions: granted ? ["schedules:manage"] : [],
      },
    },
    status: "authenticated",
  });
}

describe("CarePlanImpact", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    grantPlanEditing(true);
  });

  it("lists adjusted and hand-maintained Regeltermine with their weekdays", () => {
    renderImpact();

    expect(screen.getByText("Automatisch angepasst")).toBeInTheDocument();
    expect(screen.getByText(/freitags entfernt/)).toBeInTheDocument();
    expect(
      screen.getByText("Nicht angepasst, weil von Hand gepflegt"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Kind bleibt eingeplant/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Im Betreuungsplan ansehen" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("/betreuungsplan?view=serien&d=2027-05-31"),
    );
  });

  it("removes the child only after the two-step confirmation and marks the entry", async () => {
    mocks.removeStudentTemplateDays.mockResolvedValue(undefined);
    renderImpact();

    fireEvent.click(
      screen.getByRole("button", { name: "Dort auch entfernen" }),
    );
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText(/Neue Termine nehmen das Kind wieder auf/),
    ).toBeInTheDocument();
    expect(mocks.removeStudentTemplateDays).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("button", { name: "Entfernen" }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Jetzt entfernen" }),
    );

    await waitFor(() =>
      expect(mocks.removeStudentTemplateDays).toHaveBeenCalledWith({
        templateId: "9",
        studentId: "42",
        weekdays: [5],
        effectiveFrom: "2027-05-31",
      }),
    );
    expect(await screen.findByText("Entfernt")).toBeInTheDocument();
    expect(screen.getByText(/Kind entfernt\./)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Dort auch entfernen" }),
    ).not.toBeInTheDocument();
  });

  it("says who may remove the child when the person cannot edit the plan", () => {
    grantPlanEditing(false);
    renderImpact();

    expect(
      screen.getByText(
        "Entfernen kann nur, wer den Betreuungsplan bearbeiten darf.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Dort auch entfernen" }),
    ).not.toBeInTheDocument();
  });

  it("confirms briefly when nothing in the plan changed", () => {
    renderImpact({
      effectiveFrom: "2027-05-31",
      adjusted: [],
      notAdjusted: [],
    });

    expect(
      screen.getByText(
        "Im Betreuungsplan musste kein Regeltermin angepasst werden.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Im Betreuungsplan musste kein Regeltermin angepasst werden.",
      ),
    ).toBeVisible();
  });
});
