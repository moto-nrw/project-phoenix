import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { NotificationBadge } from "./notification-badge";

describe("NotificationBadge", () => {
  it("uses the orange tone and provided accessible label", () => {
    render(
      <NotificationBadge
        count={2}
        tone="orange"
        ariaLabel="2 offene Abwesenheitsanträge"
      />,
    );

    // -strong, not the raw brand orange: white on #F78C10 is 2.41:1, under
    // even the 3:1 non-text floor.
    expect(screen.getByLabelText("2 offene Abwesenheitsanträge")).toHaveClass(
      "bg-moto-orange-strong",
      "text-white",
    );
  });

  it("uses the blue tone and caps the visible count", () => {
    render(
      <NotificationBadge
        count={120}
        tone="blue"
        ariaLabel="120 ungelesene Nachrichten"
      />,
    );

    expect(screen.getByLabelText("120 ungelesene Nachrichten")).toHaveClass(
      "bg-moto-blue-strong",
      "text-white",
    );
    expect(screen.getByText("99+")).toBeInTheDocument();
  });

  it("uses the coral tone", () => {
    render(
      <NotificationBadge
        count={3}
        tone="coral"
        ariaLabel="3 neue Feedbackeinträge"
      />,
    );

    expect(screen.getByLabelText("3 neue Feedbackeinträge")).toHaveClass(
      "bg-moto-coral-strong",
      "text-white",
    );
  });

  it("renders nothing for an empty count", () => {
    const { container } = render(
      <NotificationBadge count={0} tone="orange" ariaLabel="Keine" />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it.each([
    ["red", "bg-moto-red"],
    ["blue", "bg-moto-blue-strong"],
  ] as const)("shows the %s tone with white text", (tone, fill) => {
    render(<NotificationBadge count={3} tone={tone} ariaLabel="3 offen" />);

    expect(screen.getByLabelText("3 offen")).toHaveClass(fill, "text-white");
  });
});
