// #6593: Während eine geplante Abwesenheit gespeichert oder entfernt wird,
// bleibt das Panel zu; eine späte Antwort landet in keinem neueren Vorgang.
// Echter SlideOver mit vaul-Mock, echte Lösch-Bestätigung.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { useState, type ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import {
  StudentStatusDayConflictError,
  type StudentStatusDay,
} from "~/lib/student-status-days-api";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));
// Die Zeitraumfelder als native Eingaben, damit der Test Tage setzen kann.
vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));

import { PlannedStatusDaysModal } from "./planned-status-days-modal";

const TITLE = "Krankmeldung planen";
const CLOSE = "Fenster schließen";

const plannedDay: StudentStatusDay = {
  id: "1",
  student_id: "42",
  date: "2026-09-15",
  status: "sick",
  label: "Krank",
  reported_at: "2026-09-08T08:00:00Z",
  cleared_at: null,
  source: "planned",
  created_at: "2026-09-08T08:00:00Z",
  updated_at: "2026-09-08T08:00:00Z",
};

type Props = ComponentProps<typeof PlannedStatusDaysModal>;

function renderPanel(props: Partial<Props> = {}) {
  const onClose = vi.fn();
  const panel = (next: Partial<Props> = {}) => (
    <PlannedStatusDaysModal
      isOpen
      status="sick"
      studentName="Kevin Anders"
      isSubmitting={false}
      existingDays={[]}
      onClose={onClose}
      loadExistingDays={() => Promise.resolve([])}
      onSubmit={vi.fn().mockResolvedValue(undefined)}
      {...props}
      {...next}
    />
  );
  const view = render(panel());
  return { ...view, panel, onClose };
}

/** Wählt einen Zeitraum und wartet, bis seine Tage geprüft sind. */
async function chooseRange(): Promise<HTMLElement> {
  fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
  fireEvent.change(screen.getByLabelText("Von"), {
    target: { value: "2026-09-21" },
  });
  fireEvent.change(screen.getByLabelText("Bis"), {
    target: { value: "2026-09-22" },
  });
  const submit = screen.getByRole("button", { name: "Krankmelden" });
  await waitFor(() => expect(submit).toBeEnabled());
  return submit;
}

async function chooseRangeAndSubmit() {
  const submit = await chooseRange();
  fireEvent.change(screen.getByLabelText("Grund (optional)"), {
    target: { value: "Arzttermin" },
  });
  fireEvent.click(submit);
}

describe("PlannedStatusDaysModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the panel shut while saving and unlocks it after a failure", async () => {
    const answer = deferred();
    const { onClose } = renderPanel({ onSubmit: vi.fn(() => answer.promise) });
    await chooseRangeAndSubmit();

    expectSlideOverSaveLocked(TITLE, ["Abbrechen", "Krankmelden"], CLOSE);
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Die Krankmeldung ist gerade nicht erreichbar.",
    );
    expectSlideOverUnlocked(TITLE, CLOSE);
    expect(screen.getByLabelText("Grund (optional)")).toHaveValue("Arzttermin");
    expect(screen.getByLabelText("Von")).toHaveValue("2026-09-21");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes itself after a successful save", async () => {
    const answer = deferred();
    function Page() {
      const [open, setOpen] = useState(true);
      return (
        <>
          <p>Kinderseite</p>
          <PlannedStatusDaysModal
            isOpen={open}
            status="sick"
            studentName="Kevin Anders"
            isSubmitting={false}
            existingDays={[]}
            onClose={() => setOpen(false)}
            loadExistingDays={() => Promise.resolve([])}
            onSubmit={() => answer.promise}
          />
        </>
      );
    }
    render(<Page />);
    await chooseRangeAndSubmit();
    expectSlideOverSaveLocked(TITLE, ["Abbrechen", "Krankmelden"], CLOSE);

    await act(async () => {
      answer.resolve();
    });

    // Anker: die Kinderseite ist wieder zu sehen.
    expect(screen.getByText("Kinderseite")).toBeVisible();
    expect(
      screen.queryByRole("heading", { name: TITLE }),
    ).not.toBeInTheDocument();
  });

  it("does not close a reopened panel on a late success", async () => {
    const answer = deferred();
    const { rerender, panel, onClose } = renderPanel({
      onSubmit: vi.fn(() => answer.promise),
    });
    await chooseRangeAndSubmit();

    // Die Kinderseite schließt und öffnet das Panel erneut, etwa für eine
    // andere Abwesenheitsart, bevor die alte Antwort ankommt.
    rerender(panel({ isOpen: false }));
    rerender(panel());
    await chooseRange();

    await act(async () => {
      answer.resolve();
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("heading", { name: TITLE })).toBeVisible();
  });

  it("shows no late failure in a reopened panel", async () => {
    const answer = deferred();
    const loadExistingDays = vi.fn(() => Promise.resolve([]));
    const { rerender, panel, onClose } = renderPanel({
      onSubmit: vi.fn(() => answer.promise),
      loadExistingDays,
    });
    await chooseRangeAndSubmit();

    rerender(panel({ isOpen: false }));
    rerender(panel());
    expectSlideOverUnlocked(TITLE, CLOSE);
    await chooseRange();
    const checksBefore = loadExistingDays.mock.calls.length;

    // Ein Konflikt meldet sich sofort im Formular und prüft die Tage neu,
    // aber nur im Vorgang, der gespeichert hat.
    await act(async () => {
      answer.reject(
        new StudentStatusDayConflictError([
          { ...plannedDay, date: "2026-09-21" },
        ]),
      );
    });

    expect(loadExistingDays).toHaveBeenCalledTimes(checksBefore);

    // Anker: das neu geöffnete Panel ist leer und sichtbar.
    expect(screen.getByLabelText("Grund (optional)")).toHaveValue("");
    expect(screen.getByRole("heading", { name: TITLE })).toBeVisible();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("returns to an unlocked panel after a planned day was removed", async () => {
    const answer = deferred();
    const onDeleteStatusDay = vi.fn(() => answer.promise);
    renderPanel({ existingDays: [plannedDay], onDeleteStatusDay });

    fireEvent.click(screen.getByRole("button", { name: /^Aktionen für/ }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Entfernen bestätigen" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Eintrag entfernen" }));
    expect(onDeleteStatusDay).toHaveBeenCalledWith("1");
    expect(
      screen.getByRole("heading", { name: "Geplanten Tag entfernen?" }),
    ).toBeVisible();

    await act(async () => {
      answer.resolve();
    });

    await waitFor(() => expectSlideOverUnlocked(TITLE, CLOSE));
  });
});
