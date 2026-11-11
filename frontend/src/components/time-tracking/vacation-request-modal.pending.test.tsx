// #6593: Während der Urlaubsantrag gesendet wird, bleibt das Panel zu; eine
// späte Antwort leert und schließt keinen neuen Antrag. Echter SlideOver mit
// vaul-Mock.
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import type { DateRange } from "react-day-picker";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { setTestClock } from "~/test/clock";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const mocks = vi.hoisted(() => ({
  requestVacation: vi.fn(),
  toastSuccess: vi.fn(),
}));

// Der Kalender wählt per Knopfdruck einen festen Zeitraum (Mo–Di).
vi.mock("~/components/ui/date-range-picker", () => ({
  RangeCalendarInline: ({
    onChange,
  }: {
    readonly onChange: (range: DateRange | undefined) => void;
  }) => (
    <button
      type="button"
      onClick={() =>
        onChange({ from: new Date(2027, 6, 5), to: new Date(2027, 6, 6) })
      }
    >
      Test-Zeitraum wählen
    </button>
  ),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ error: vi.fn(), success: mocks.toastSuccess }),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { requestVacation: mocks.requestVacation },
}));

import { VacationRequestModal } from "./vacation-request-modal";

const TITLE = "Urlaub beantragen";
const LOCKED_FOOTER = ["Abbrechen", "Wird gesendet…"] as const;

type Props = ComponentProps<typeof VacationRequestModal>;

function renderAndSend() {
  const answer = deferred<unknown>();
  mocks.requestVacation.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onSubmitted = vi.fn();
  const modal = (next: Partial<Props> = {}) => (
    <VacationRequestModal
      isOpen
      onClose={onClose}
      onSubmitted={onSubmitted}
      remainingDays={20}
      existingVacations={[]}
      {...next}
    />
  );
  const view = render(modal());
  fireEvent.click(screen.getByRole("button", { name: "Test-Zeitraum wählen" }));
  fireEvent.change(screen.getByRole("textbox"), {
    target: { value: "Vertretung geklärt" },
  });
  const send = screen.getByRole("button", { name: "Antrag senden" });
  expect(send).toBeEnabled();
  fireEvent.click(send);
  return { ...view, answer, modal, onClose, onSubmitted };
}

describe("VacationRequestModal pending lock (#6593)", () => {
  beforeEach(() => {
    setTestClock(new Date(2027, 6, 1, 10, 0, 0));
    vi.clearAllMocks();
  });

  it("keeps the panel shut while sending and unlocks it after a failure", async () => {
    const { answer, onClose } = renderAndSend();

    expect(mocks.requestVacation).toHaveBeenCalledOnce();
    expectSlideOverSaveLocked(TITLE, LOCKED_FOOTER);
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Die Urlaubsanfrage ist gerade nicht erreichbar.",
    );
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByRole("textbox")).toHaveValue("Vertretung geklärt");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("reports the request and closes the panel on success", async () => {
    const { answer, onClose, onSubmitted } = renderAndSend();

    await act(async () => {
      answer.resolve({});
    });

    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      "Der Urlaubsantrag ist gesendet.",
    );
    expect(onSubmitted).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("refreshes but neither clears nor closes a reopened panel on a late answer", async () => {
    const { answer, onClose, onSubmitted, rerender, modal } = renderAndSend();

    rerender(modal({ isOpen: false }));
    rerender(modal());
    expectSlideOverUnlocked(TITLE);
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "Neuer Antrag" },
    });

    await act(async () => {
      answer.resolve({});
    });

    expect(onSubmitted).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("textbox")).toHaveValue("Neuer Antrag");
  });
});
