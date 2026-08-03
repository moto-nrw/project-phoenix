import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  ConflictWarningsBanner,
  type ConflictBannerEntry,
} from "./conflict-warnings-banner";

const studentConflict: ConflictBannerEntry = {
  fingerprint: "aaaa1111bbbb2222cccc3333dddd4444",
  kind: "student",
  personName: "Emma Beispiel",
  dateISO: "2026-06-22",
  overlapStart: "14:30",
  overlapEnd: "15:00",
  instances: [
    { id: "1", title: "Fußball-AG" },
    { id: "2", title: "Lernzeit" },
  ],
};

const staffConflict: ConflictBannerEntry = {
  fingerprint: "eeee5555ffff6666aaaa7777bbbb8888",
  kind: "staff",
  personName: "Maria Muster",
  dateISO: "2026-06-23",
  overlapStart: "13:00",
  overlapEnd: "14:00",
  instances: [
    { id: "3", title: "Hausaufgaben" },
    { id: "4", title: "Bastel-AG" },
  ],
};

function renderBanner(
  props: Partial<React.ComponentProps<typeof ConflictWarningsBanner>> = {},
) {
  const defaults = {
    openConflicts: [] as ConflictBannerEntry[],
    hiddenConflicts: [] as ConflictBannerEntry[],
    periodLabel: "diese Woche",
    onHide: vi.fn(),
    onUnhide: vi.fn(),
    onJump: vi.fn(),
  };
  const merged = { ...defaults, ...props };
  return { ...render(<ConflictWarningsBanner {...merged} />), props: merged };
}

describe("ConflictWarningsBanner", () => {
  it("renders nothing without open or hidden conflicts", () => {
    const { container } = renderBanner();

    expect(container).toBeEmptyDOMElement();
  });

  it("counts only open conflicts and names the checked period", () => {
    renderBanner({
      openConflicts: [studentConflict],
      hiddenConflicts: [staffConflict],
    });

    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("1 Konflikt");
    expect(status).toHaveTextContent("diese Woche");
    expect(status).toHaveTextContent("1 ausgeblendet");
  });

  it("uses singular and plural labels", () => {
    const { rerender, props } = renderBanner({
      openConflicts: [studentConflict],
    });

    expect(screen.getByRole("status")).toHaveTextContent("1 Konflikt");

    rerender(
      <ConflictWarningsBanner
        {...props}
        openConflicts={[studentConflict, staffConflict]}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("2 Konflikte");
  });

  it("names the month window when the month view is checked", () => {
    renderBanner({
      openConflicts: [studentConflict],
      periodLabel: "in diesem Monat",
    });

    expect(screen.getByRole("status")).toHaveTextContent("in diesem Monat");
  });

  it("expands to conflict details with person, kind, and involved blocks", () => {
    renderBanner({ openConflicts: [studentConflict] });

    fireEvent.click(
      screen.getByRole("button", { name: "Konfliktdetails anzeigen" }),
    );

    expect(screen.getByText("Emma Beispiel")).toBeInTheDocument();
    expect(screen.getByText(/\(Kind\) doppelt eingeplant/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Termin „Fußball-AG“ öffnen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Termin „Lernzeit“ öffnen" }),
    ).toBeInTheDocument();
  });

  it("hides a conflict via the accessible Ausblenden action", () => {
    const { props } = renderBanner({ openConflicts: [studentConflict] });

    fireEvent.click(
      screen.getByRole("button", { name: "Konfliktdetails anzeigen" }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: "Konflikt für Emma Beispiel ausblenden",
      }),
    );

    expect(props.onHide).toHaveBeenCalledWith(studentConflict);
  });

  it("reveals hidden conflicts and restores them", () => {
    const { props } = renderBanner({ hiddenConflicts: [staffConflict] });

    expect(screen.getByRole("status")).toHaveTextContent(
      "Keine offenen Konflikte diese Woche",
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Konfliktdetails anzeigen" }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: "Ausgeblendete Konflikte anzeigen (1)",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: "Konflikt für Maria Muster wieder anzeigen",
      }),
    );

    expect(props.onUnhide).toHaveBeenCalledWith(staffConflict);
  });

  it("jumps to an involved block", () => {
    const { props } = renderBanner({ openConflicts: [studentConflict] });

    fireEvent.click(
      screen.getByRole("button", { name: "Konfliktdetails anzeigen" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Termin „Lernzeit“ öffnen" }),
    );

    expect(props.onJump).toHaveBeenCalledWith(studentConflict, "2");
  });

  it("is keyboard operable: actions are real buttons reachable in order", () => {
    renderBanner({
      openConflicts: [studentConflict],
      hiddenConflicts: [staffConflict],
    });

    const toggle = screen.getByRole("button", {
      name: "Konfliktdetails anzeigen",
    });
    toggle.focus();
    expect(toggle).toHaveFocus();
    expect(toggle).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");

    const hide = screen.getByRole("button", {
      name: "Konflikt für Emma Beispiel ausblenden",
    });
    hide.focus();
    expect(hide).toHaveFocus();
  });
});
