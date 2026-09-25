import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/dashboard/modal-context";
import { SeriesClosingDaysChoiceModal } from "./series-closing-days-choice-modal";

function renderPrompt(currentChoice?: boolean) {
  const onChoose = vi.fn();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ModalProvider>{children}</ModalProvider>
  );
  render(
    <SeriesClosingDaysChoiceModal
      closingDayCount={2}
      currentChoice={currentChoice}
      onCancel={vi.fn()}
      onChoose={onChoose}
    />,
    { wrapper },
  );
  return { onChoose };
}

const skipButton = () =>
  screen.getByRole("button", { name: /Schließtage auslassen/ });
const includeButton = () =>
  screen.getByRole("button", { name: /Auch an Schließtagen planen/ });

describe("SeriesClosingDaysChoiceModal (#3594)", () => {
  it("marks no option for a new series", () => {
    renderPrompt();

    expect(screen.queryByText(/Wie bisher/)).not.toBeInTheDocument();
    expect(skipButton().className).toBe(includeButton().className);
  });

  it("highlights the stored choice of an edited series", () => {
    const { onChoose } = renderPrompt(true);

    expect(includeButton()).toHaveTextContent(
      "Wie bisher. Zum Beispiel für die Ferienbetreuung.",
    );
    expect(skipButton()).not.toHaveTextContent("Wie bisher");
    expect(includeButton().className).not.toBe(skipButton().className);

    fireEvent.click(includeButton());
    expect(onChoose).toHaveBeenCalledWith(true);
  });

  it("highlights skipping when the series skips closing days so far", () => {
    renderPrompt(false);

    expect(skipButton()).toHaveTextContent(
      "Wie bisher. An Schließtagen fällt der Termin aus.",
    );
    expect(includeButton()).not.toHaveTextContent("Wie bisher");
  });
});
