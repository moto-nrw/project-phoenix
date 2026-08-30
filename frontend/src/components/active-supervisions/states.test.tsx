import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SchulhofNotSupervisingView } from "./states";

describe("SchulhofNotSupervisingView", () => {
  it("keeps an empty-yard weekend start visible and explains why it is blocked", () => {
    render(
      <SchulhofNotSupervisingView
        supervisorCount={0}
        supervisorNames={[]}
        isToggling={false}
        startDisabled
        startDisabledReason="Spontane Aktivitäten sind nur montags bis freitags möglich."
        onToggle={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Beaufsichtigen" }),
    ).toBeDisabled();
    expect(
      screen.getByText(
        "Spontane Aktivitäten sind nur montags bis freitags möglich.",
      ),
    ).toBeInTheDocument();
  });
});
