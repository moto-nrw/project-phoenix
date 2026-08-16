import {
  BuildingsIcon,
  FirstAidKitIcon,
  ParkIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MotoDuotoneIcon } from "./moto-duotone-icon";

describe("MotoDuotoneIcon", () => {
  it("uses the fixed website blue pair", () => {
    const { container } = render(
      <MotoDuotoneIcon icon={BuildingsIcon} tone="blue" />,
    );
    const icon = container.querySelector("svg");

    expect(icon).toHaveAttribute("data-moto-duotone-tone", "blue");
    expect(icon).toHaveStyle({ color: "#315C9B" });
    expect(icon?.style.getPropertyValue("--moto-icon-secondary")).toBe(
      "#6B95E0",
    );
    expect(icon?.querySelector('[opacity="0.2"]')).toBeInTheDocument();
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  it("uses the work-time chart blue for time tracking", () => {
    const { container } = render(
      <MotoDuotoneIcon icon={BuildingsIcon} tone="timeTracking" />,
    );
    const icon = container.querySelector("svg");

    expect(icon).toHaveStyle({ color: "#0EA5E9" });
    expect(icon?.style.getPropertyValue("--moto-icon-secondary")).toBe(
      "#7DD3FC",
    );
  });

  it("uses both website greens for group icons", () => {
    const { container } = render(
      <MotoDuotoneIcon icon={UsersThreeIcon} tone="greenDeep" />,
    );
    const icon = container.querySelector("svg");

    expect(icon).toHaveStyle({ color: "#3F6F12" });
    expect(icon?.style.getPropertyValue("--moto-icon-secondary")).toBe(
      "#92D63C",
    );
  });

  it("uses the yellow website pair for the schoolyard", () => {
    const { container } = render(
      <MotoDuotoneIcon icon={ParkIcon} tone="amber" />,
    );
    const icon = container.querySelector("svg");

    expect(icon).toHaveStyle({ color: "#EAB308" });
    expect(icon?.style.getPropertyValue("--moto-icon-secondary")).toBe(
      "#FACC15",
    );
  });

  it("uses a red and white pair for medical icons", () => {
    const { container } = render(
      <MotoDuotoneIcon icon={FirstAidKitIcon} tone="red" />,
    );
    const icon = container.querySelector("svg");

    expect(icon).toHaveStyle({ color: "#B91C1C" });
    expect(icon?.style.getPropertyValue("--moto-icon-secondary")).toBe(
      "var(--color-white)",
    );
  });

  it("exposes an accessible name when the icon carries meaning", () => {
    render(<MotoDuotoneIcon icon={BuildingsIcon} tone="teal" label="Räume" />);

    expect(screen.getByRole("img", { name: "Räume" })).toBeInTheDocument();
  });

  it("can render a compact icon without the duotone background fill", () => {
    const { container } = render(
      <MotoDuotoneIcon icon={BuildingsIcon} tone="blue" weight="regular" />,
    );
    const icon = container.querySelector("svg");

    expect(icon?.querySelector('[opacity="0.2"]')).not.toBeInTheDocument();
  });
});
