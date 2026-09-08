import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { OpenRoomNotice, SchulhofSuperviseButton } from "./states";

describe("OpenRoomNotice", () => {
  it("says the room is shared and that the reader has no supervision here", () => {
    // The block this replaced hid the children from everyone who was not
    // supervising. A released room is open to all caregivers, so the notice
    // separates "shared" from "mine" instead of gating the view (#3065).
    render(<OpenRoomNotice isUserSupervising={false} />);

    expect(
      screen.getByText(/Diesen Raum sehen alle Betreuungskräfte/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Sie haben hier keine Aufsicht/),
    ).toBeInTheDocument();
  });

  it("names the supervisors where the screen knows them", () => {
    render(
      <OpenRoomNotice
        isUserSupervising={false}
        supervisorNames={["Anna Meier", "Ben Klein"]}
      />,
    );

    expect(
      screen.getByText(/Die Aufsicht hat: Anna Meier, Ben Klein/),
    ).toBeInTheDocument();
  });

  it("states the caller's own supervision as such", () => {
    render(<OpenRoomNotice isUserSupervising />);

    expect(screen.getByText(/Sie haben hier die Aufsicht/)).toBeInTheDocument();
  });

  it("keeps a blocked action's reason next to the action", () => {
    render(
      <OpenRoomNotice
        isUserSupervising={false}
        hint="Spontane Aktivitäten sind nur montags bis freitags möglich."
        action={
          <SchulhofSuperviseButton
            isToggling={false}
            disabled
            onToggle={vi.fn()}
          />
        }
      />,
    );

    expect(
      screen.getByRole("button", { name: "Beaufsichtigen" }),
    ).toBeDisabled();
    expect(
      screen.getByText(
        /Spontane Aktivitäten sind nur montags bis freitags möglich/,
      ),
    ).toBeInTheDocument();
  });
});
