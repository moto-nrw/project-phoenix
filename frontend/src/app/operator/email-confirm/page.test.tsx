import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

vi.mock("./email-confirm-content", () => ({
  EmailConfirmContent: () => (
    <div data-testid="email-confirm-content">Mocked</div>
  ),
}));

import OperatorEmailConfirmPage from "./page";

describe("OperatorEmailConfirmPage", () => {
  it("renders EmailConfirmContent", () => {
    render(<OperatorEmailConfirmPage />);
    expect(screen.getByTestId("email-confirm-content")).toBeInTheDocument();
  });
});
