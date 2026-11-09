import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PushInstallSteps } from "./push-install-steps";

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

describe("PushInstallSteps", () => {
  it("keeps the previous vertical padding for the normal variant", () => {
    render(<PushInstallSteps />);

    for (const item of screen.getAllByRole("listitem")) {
      expect(item.lastElementChild).toHaveClass("py-2.5");
      expect(item.lastElementChild).not.toHaveClass("py-2");
    }
  });

  it("uses compact padding for the compact variant", () => {
    render(<PushInstallSteps compact />);

    for (const item of screen.getAllByRole("listitem")) {
      expect(item.lastElementChild).toHaveClass("py-2");
      expect(item.lastElementChild).not.toHaveClass("py-2.5");
    }
  });
});
