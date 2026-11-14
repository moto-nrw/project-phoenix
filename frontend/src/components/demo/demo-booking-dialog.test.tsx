import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { calendarScale, DemoBookingDialog } from "./demo-booking-dialog";

const ORIGIN = "https://meetings-eu1.hubspot.com";

function renderCalendar() {
  render(<DemoBookingDialog isOpen onClose={() => undefined} />);
  const frame = screen.getByTitle(
    "Kalender für einen Termin mit dem moto-Team",
  );
  // The test DOM loads no child frames; the frame gets a stand-in window.
  const postMessage = vi.fn();
  const frameWindow = { postMessage } as unknown as Window;
  Object.defineProperty(frame, "contentWindow", { value: frameWindow });
  const send = (data: unknown, origin = ORIGIN) =>
    act(() => {
      window.dispatchEvent(
        new MessageEvent("message", { data, origin, source: frameWindow }),
      );
    });
  return { postMessage, send };
}

describe("DemoBookingDialog", () => {
  it("asks only the calendar's own origin not to track", () => {
    const { postMessage, send } = renderCalendar();

    send("readyForConsentListener", "https://evil.example");
    expect(postMessage).not.toHaveBeenCalled();

    send("readyForConsentListener");
    expect(postMessage).toHaveBeenCalledWith(
      { type: "privacy-consent", shouldTrackAnalytics: false },
      ORIGIN,
    );
  });

  it("shows the calendar once it reports its height", () => {
    const { send } = renderCalendar();
    expect(screen.getByText("Der Kalender lädt …")).toBeInTheDocument();

    send({ height: 640 });

    expect(screen.queryByText("Der Kalender lädt …")).toBeNull();
    expect(
      screen.getByTitle("Kalender für einen Termin mit dem moto-Team"),
    ).toBeVisible();
  });

  it("shrinks the calendar on a short wide screen, not on a phone", () => {
    expect(calendarScale(900, 715, 715)).toBe(1);
    expect(calendarScale(900, 710, 715)).toBe(1);
    expect(calendarScale(900, 572, 715)).toBe(0.8);
    expect(calendarScale(400, 500, 715)).toBe(1);
  });

  it("keeps the calendar at full size when shrinking would make it unreadable", () => {
    expect(calendarScale(900, 430, 715)).toBe(0.601);
    expect(calendarScale(900, 420, 715)).toBe(1);
    expect(calendarScale(900, 198, 715)).toBe(1);
  });
});
