import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SchulhofSuperviseButton } from "./states";

describe("SchulhofSuperviseButton", () => {
  it("takes the Schulhof supervision on click", () => {
    const onToggle = vi.fn();
    render(<SchulhofSuperviseButton isToggling={false} onToggle={onToggle} />);

    fireEvent.click(screen.getByRole("button", { name: "Beaufsichtigen" }));

    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("stays disabled while the supervision is being taken", () => {
    render(<SchulhofSuperviseButton isToggling onToggle={vi.fn()} />);

    expect(
      screen.getByRole("button", { name: "Wird übernommen…" }),
    ).toBeDisabled();
  });

  it("is disabled when the action is blocked", () => {
    render(
      <SchulhofSuperviseButton
        isToggling={false}
        disabled
        onToggle={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Beaufsichtigen" }),
    ).toBeDisabled();
  });
});
