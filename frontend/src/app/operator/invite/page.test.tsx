import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

vi.mock("./invite-content", () => ({
  InviteContent: () => <div data-testid="invite-content">InviteContent</div>,
}));

import OperatorInvitePage from "./page";

describe("OperatorInvitePage", () => {
  it("renders InviteContent component", () => {
    const { getByTestId } = render(<OperatorInvitePage />);
    expect(getByTestId("invite-content")).toBeInTheDocument();
  });
});
