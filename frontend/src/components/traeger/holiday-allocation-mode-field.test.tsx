import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { HolidayCriterion } from "~/lib/traeger-holiday-allocation-api";

vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import { HolidayAllocationModeField } from "./holiday-allocation-mode-field";

const CRITERION: HolidayCriterion = {
  id: "1",
  name: "Beide Eltern berufstätig",
  question: "Sind beide Eltern berufstätig?",
  scope: "family",
  active: true,
  position: 1,
  answered: false,
};

describe("allocation mode of a holiday period (#4700)", () => {
  it("switches to criteria and names the active criteria with the link to them", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <HolidayAllocationModeField
        value="arrival"
        locked={false}
        criteria={[CRITERION]}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByRole("radio", { name: /Nach Kriterien/ }));
    expect(onChange).toHaveBeenCalledWith("criteria");

    rerender(
      <HolidayAllocationModeField
        value="criteria"
        locked={false}
        criteria={[CRITERION]}
        onChange={onChange}
      />,
    );
    expect(
      screen.getByText(
        /1 Kriterium ist eingeschaltet: 1\. Beide Eltern berufstätig\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Kriterien festlegen" }),
    ).toHaveAttribute("href", "/traeger/ferien/kriterien");
  });

  it("warns without an active criterion", () => {
    render(
      <HolidayAllocationModeField
        value="criteria"
        locked={false}
        criteria={[{ ...CRITERION, active: false }]}
        onChange={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/Es ist noch kein Kriterium eingeschaltet/),
    ).toBeInTheDocument();
  });

  it("keeps the choice once the period has registrations", () => {
    render(
      <HolidayAllocationModeField value="arrival" locked onChange={vi.fn()} />,
    );
    expect(
      screen.getByText(
        "Es gibt schon Anmeldungen. Darum bleibt die Platzvergabe so.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Nach Kriterien/ }),
    ).toBeDisabled();
  });
});
