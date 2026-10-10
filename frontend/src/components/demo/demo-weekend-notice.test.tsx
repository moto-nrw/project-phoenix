import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { beforeEach, describe, expect, it } from "vitest";
import { ModalProvider, useModal } from "~/components/dashboard/modal-context";
import { setTestClock } from "~/test/clock";
import { DemoWeekendNotice, isWeekendDay } from "./demo-weekend-notice";

describe("isWeekendDay", () => {
  it("reads Saturday and Sunday as weekend", () => {
    expect(isWeekendDay("2026-09-26")).toBe(true);
    expect(isWeekendDay("2026-09-27")).toBe(true);
    expect(isWeekendDay("2026-09-25")).toBe(false);
    expect(isWeekendDay("2026-09-28")).toBe(false);
  });
});

describe("DemoWeekendNotice", () => {
  beforeEach(() => {
    globalThis.localStorage.clear();
  });

  it("stays hidden on a weekday", () => {
    render(<DemoWeekendNotice inParentsApp={false} />);
    expect(
      screen.queryByRole("button", { name: "Hinweis zum Wochenende" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Heute ist Wochenende")).not.toBeInTheDocument();
  });

  it("explains the weekend on Friday's plan once per weekend day", async () => {
    setTestClock("2026-09-26T11:00:00+02:00");
    const user = userEvent.setup();
    const { unmount } = render(<DemoWeekendNotice inParentsApp={false} />);

    expect(
      await screen.findByText("Heute ist Wochenende", {}, { timeout: 4000 }),
    ).toBeInTheDocument();
    // The demo runs its weekend on Friday's plan (#3921): nobody is
    // "Ungeplant anwesend" any more, so the notice no longer explains that.
    expect(
      screen.getByText("Am Wochenende gilt in der Demo der Plan vom Freitag."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Ungeplant anwesend/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Verstanden" }));
    unmount();

    // Next page view the same day: no dialog, but the button reopens it.
    render(<DemoWeekendNotice inParentsApp={false} />);
    expect(screen.queryByText("Heute ist Wochenende")).not.toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Hinweis zum Wochenende" }),
    );
    expect(await screen.findByText("Heute ist Wochenende")).toBeInTheDocument();
  });

  it("speaks about the own child in the parents app", async () => {
    setTestClock("2026-09-27T09:00:00+02:00");
    render(<DemoWeekendNotice inParentsApp />);

    expect(
      await screen.findByText(
        "Deshalb ist Ihr Kind heute in der OGS.",
        {},
        { timeout: 4000 },
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Ungeplant anwesend/)).not.toBeInTheDocument();
  });

  it("waits while another dialog is open", async () => {
    setTestClock("2026-09-26T11:00:00+02:00");
    const user = userEvent.setup();
    let close = () => undefined as void;
    function OtherDialog() {
      const { openModal, closeModal } = useModal();
      useEffect(() => {
        openModal();
        close = closeModal;
      }, [openModal, closeModal]);
      return null;
    }
    render(
      <ModalProvider>
        <OtherDialog />
        <DemoWeekendNotice inParentsApp={false} />
      </ModalProvider>,
    );

    await user.click(
      screen.getByRole("button", { name: "Hinweis zum Wochenende" }),
    );
    expect(screen.queryByText("Heute ist Wochenende")).not.toBeInTheDocument();
    await act(() => new Promise((resolve) => setTimeout(resolve, 2000)));
    expect(screen.queryByText("Heute ist Wochenende")).not.toBeInTheDocument();
    act(() => close());
    expect(await screen.findByText("Heute ist Wochenende")).toBeInTheDocument();
  });
});
