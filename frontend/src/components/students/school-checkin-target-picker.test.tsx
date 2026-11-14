import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { CheckinTargetState } from "~/lib/hooks/use-checkin-target";
import type { TargetRoomOption } from "~/lib/move-targets";
import { SchoolCheckinTargetPicker } from "./school-checkin-target-picker";

// „Anmelden in“ of the check-in mode (#6953).

const mensa: TargetRoomOption = {
  kind: "session",
  value: "12",
  activeGroupId: "12",
  roomId: "3",
  roomName: "Mensa",
  activityName: "Mittagessen",
  label: "Mensa",
};

function state(
  overrides: Partial<CheckinTargetState> = {},
): CheckinTargetState {
  return {
    available: true,
    options: [mensa],
    loading: false,
    loadError: undefined,
    target: null,
    setTargetValue: vi.fn(),
    lostTargetName: null,
    dismissLost: vi.fn(),
    refresh: vi.fn(async () => undefined),
    ...overrides,
  };
}

describe("SchoolCheckinTargetPicker", () => {
  it("says that children go without a room by default", () => {
    render(<SchoolCheckinTargetPicker state={state()} />);
    expect(
      screen.getByRole("combobox", { name: "Anmelden in" }),
    ).toHaveTextContent("Anwesend (ohne Raum)");
    expect(
      screen.getByText(
        "Kinder stehen nach dem Anmelden auf „Unterwegs“, bis sie in einem Raum sind.",
      ),
    ).toBeInTheDocument();
  });

  it("names the chosen room", () => {
    render(<SchoolCheckinTargetPicker state={state({ target: mensa })} />);
    expect(
      screen.getByRole("combobox", { name: "Anmelden in" }),
    ).toHaveTextContent("Mensa · Mittagessen");
    expect(
      screen.getByText(/Kinder sind nach dem Anmelden sofort in/),
    ).toHaveTextContent("Kinder sind nach dem Anmelden sofort in Mensa.");
  });

  it("explains a target that is gone and lets the person close it", () => {
    const dismissLost = vi.fn();
    render(
      <SchoolCheckinTargetPicker
        state={state({ lostTargetName: "Werkraum", dismissLost })}
      />,
    );
    expect(
      screen.getByText(/In Werkraum läuft keine Aufsicht mehr/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /schließen/i }));
    expect(dismissLost).toHaveBeenCalledOnce();
  });

  it("shows no choice where the school has no rooms to choose", () => {
    const { container } = render(
      <SchoolCheckinTargetPicker state={state({ available: false })} />,
    );
    expect(container).toBeEmptyDOMElement();
    expect(
      screen.getAllByText((_, node) => node === document.body),
    ).toHaveLength(1);
  });
});
