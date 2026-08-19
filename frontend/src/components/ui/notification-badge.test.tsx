import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { NotificationBadge } from "./notification-badge";

describe("NotificationBadge", () => {
  it("uses the staff tone and provided accessible label", () => {
    render(
      <NotificationBadge
        count={2}
        tone="staff"
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

  it("uses the parents tone and caps the visible count", () => {
    render(
      <NotificationBadge
        count={120}
        tone="parents"
        ariaLabel="120 ungelesene Nachrichten"
      />,
    );

    expect(screen.getByLabelText("120 ungelesene Nachrichten")).toHaveClass(
      "bg-moto-blue-strong",
      "text-white",
    );
    expect(screen.getByText("99+")).toBeInTheDocument();
  });

  it("renders nothing for an empty count", () => {
    const { container } = render(
      <NotificationBadge count={0} tone="staff" ariaLabel="Keine" />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});
