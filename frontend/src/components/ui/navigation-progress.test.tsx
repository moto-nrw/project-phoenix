import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

const linkStatus = vi.hoisted(() => ({ pending: false }));

vi.mock("next/link", () => ({
  useLinkStatus: () => linkStatus,
  default: ({
    children,
    href,
  }: {
    children?: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

import {
  NavigationProgressBar,
  NavigationProgressProvider,
  NavigationProgressReporter,
} from "./navigation-progress";

function renderShell() {
  return render(
    <NavigationProgressProvider>
      <NavigationProgressBar />
      <NavigationProgressReporter />
    </NavigationProgressProvider>,
  );
}

describe("NavigationProgress", () => {
  it("shows nothing while no navigation is pending", () => {
    linkStatus.pending = false;
    renderShell();

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("");
  });

  it("shows the bar and announces the load while a link is pending", () => {
    linkStatus.pending = true;
    renderShell();

    expect(screen.getByTestId("navigation-progress")).toHaveClass(
      "moto-nav-progress",
    );
    expect(screen.getByRole("status")).toHaveTextContent("Seite wird geladen");
  });

  it("keeps the announcement region mounted so it can be read out at all", () => {
    linkStatus.pending = false;
    renderShell();

    // Ein Bereich, der erst beim Wechsel eingehängt wird, wird von
    // Screenreadern nicht angesagt — er muss vorher da sein.
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
  });

  it("renders nothing outside a provider instead of throwing", () => {
    linkStatus.pending = true;
    render(<NavigationProgressBar />);

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });
});
