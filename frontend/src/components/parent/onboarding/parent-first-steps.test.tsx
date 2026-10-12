import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ParentFirstSteps } from "./parent-first-steps";
import {
  parentFirstStepsStorageKey,
  readParentFirstStepsState,
} from "./parent-first-steps-state";

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
  useRouter: () => ({ push: vi.fn() }),
}));

describe("ParentFirstSteps", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it("shows five separate, mobile-first tours for parents", async () => {
    render(
      <ParentFirstSteps accountId="42" childCount={1} newsEnabled={true} />,
    );

    expect(
      await screen.findByRole("heading", { name: "Erste Schritte" }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(5);
    expect(
      screen.getByText("moto als App auf dem Handy hinzufügen"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Benachrichtigungen einschalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Angaben zu meinem Kind ändern",
      }),
    );
    expect(
      screen.getByText(
        "Hier finden Sie Angaben, Betreuungszeiten, Heimwege und Erziehungsberechtigte.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Anleitung öffnen" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("mein-kind-und-den-heutigen-tag-ansehen"),
    );

    const panel = screen.getByRole("region", { name: "Erste Schritte" });
    expect(panel).toHaveClass("left-2", "right-2");
    expect(panel.className).toContain("env(safe-area-inset-bottom)");
  });

  it("adapts the list when no child or parent letters are available", async () => {
    render(
      <ParentFirstSteps accountId="42" childCount={0} newsEnabled={false} />,
    );

    await screen.findByRole("heading", { name: "Erste Schritte" });
    expect(screen.getAllByRole("listitem")).toHaveLength(4);
    expect(screen.getByText("Nachrichten finden")).toBeInTheDocument();
    expect(
      screen.queryByText("Angaben zu meinem Kind ändern"),
    ).not.toBeInTheDocument();
  });

  it("persists dismissal for the parent account", async () => {
    render(
      <ParentFirstSteps accountId="42" childCount={1} newsEnabled={true} />,
    );
    await screen.findByRole("heading", { name: "Erste Schritte" });

    fireEvent.click(
      screen.getByRole("button", { name: "Erste Schritte ausblenden" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Ausblenden" }));

    await waitFor(() =>
      expect(
        screen.queryByRole("region", { name: "Erste Schritte" }),
      ).not.toBeInTheDocument(),
    );
    expect(readParentFirstStepsState("42").dismissed).toBe(true);
    expect(localStorage.getItem(parentFirstStepsStorageKey("43"))).toBeNull();
  });

  it("completes app installation when the installation guide is opened", async () => {
    render(
      <ParentFirstSteps accountId="42" childCount={1} newsEnabled={true} />,
    );
    await screen.findByRole("heading", { name: "Erste Schritte" });

    fireEvent.click(
      screen.getByRole("button", {
        name: "moto als App auf dem Handy hinzufügen",
      }),
    );
    const guide = screen.getByRole("link", { name: "Anleitung öffnen" });
    guide.addEventListener("click", (event) => event.preventDefault(), {
      once: true,
    });
    fireEvent.click(guide);

    expect(readParentFirstStepsState("42").completed).toContain("installApp");
  });
});
