import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  SchoolCheckinToggle,
  SchoolCheckinToggleMobile,
} from "./school-checkin-toggle";

describe("SchoolCheckinToggle (desktop)", () => {
  it("renders inactive label and aria-pressed=false by default", () => {
    render(<SchoolCheckinToggle isActive={false} onToggle={vi.fn()} />);

    const btn = screen.getByRole("button", { name: /Schüler An- & Abmelden/ });
    expect(btn).toHaveAttribute("aria-pressed", "false");
    expect(btn).not.toHaveAttribute("data-active");
  });

  it("renders active label and aria-pressed=true when active", () => {
    render(<SchoolCheckinToggle isActive={true} onToggle={vi.fn()} />);

    const btn = screen.getByRole("button", { name: /An-\/Abmelden beenden/ });
    expect(btn).toHaveAttribute("aria-pressed", "true");
    expect(btn).toHaveAttribute("data-active", "true");
  });

  it("fires onToggle when clicked", () => {
    const onToggle = vi.fn();
    render(<SchoolCheckinToggle isActive={false} onToggle={onToggle} />);

    fireEvent.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("shows pending count when > 0", () => {
    render(
      <SchoolCheckinToggle
        isActive={true}
        onToggle={vi.fn()}
        pendingCount={3}
      />,
    );
    expect(
      screen.getByRole("button", { name: /An-\/Abmelden beenden/ }),
    ).toHaveTextContent("(3)");
  });

  it("hides pending count when 0", () => {
    render(
      <SchoolCheckinToggle
        isActive={true}
        onToggle={vi.fn()}
        pendingCount={0}
      />,
    );
    expect(screen.getByRole("button")).not.toHaveTextContent("(0)");
  });
});

describe("SchoolCheckinToggleMobile", () => {
  it("renders with aria-label for screen readers", () => {
    render(<SchoolCheckinToggleMobile isActive={false} onToggle={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: /Schüler An- & Abmelden/ }),
    ).toBeInTheDocument();
  });

  it("shows pending count badge when > 0", () => {
    render(
      <SchoolCheckinToggleMobile
        isActive={false}
        onToggle={vi.fn()}
        pendingCount={2}
      />,
    );
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("fires onToggle on click", () => {
    const onToggle = vi.fn();
    render(<SchoolCheckinToggleMobile isActive={false} onToggle={onToggle} />);

    fireEvent.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
