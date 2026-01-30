/**
 * Tests for SessionWarning Component
 * Tests expiry warning display in desktop and mobile variants
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { SessionWarning } from "./session-warning";

describe("SessionWarning", () => {
  it("renders nothing when session is not expired", () => {
    const { container } = render(
      <SessionWarning isExpired={false} variant="desktop" />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders desktop warning when expired", () => {
    render(<SessionWarning isExpired={true} variant="desktop" />);

    expect(
      screen.getByText(
        "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
      ),
    ).toBeInTheDocument();
  });

  it("renders mobile warning when expired", () => {
    const { container } = render(
      <SessionWarning isExpired={true} variant="mobile" />,
    );

    // Mobile version shows icon only, no text
    const icon = container.querySelector("svg");
    expect(icon).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
      ),
    ).not.toBeInTheDocument();
  });

  it("desktop variant has proper styling", () => {
    const { container } = render(
      <SessionWarning isExpired={true} variant="desktop" />,
    );

    const warning = container.querySelector(".border-red-200");
    expect(warning).toBeInTheDocument();
    expect(warning).toHaveClass("bg-red-50");
  });

  it("mobile variant has proper icon color", () => {
    const { container } = render(
      <SessionWarning isExpired={true} variant="mobile" />,
    );

    const iconContainer = container.querySelector(".text-red-600");
    expect(iconContainer).toBeInTheDocument();
  });
});
