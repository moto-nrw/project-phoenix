import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import type { ReactNode } from "react";
import { ChoiceModal } from "./choice-modal";
import { ModalProvider } from "../dashboard/modal-context";

function TestWrapper({ children }: { children: ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

const options = [
  {
    value: "only_this",
    label: "Nur diesen Termin",
    description: "Andere Termine bleiben unverändert",
  },
  {
    value: "this_and_following",
    label: "Diesen und alle folgenden",
    description: "Gilt ab diesem Datum",
  },
];

describe("ChoiceModal", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does not render when isOpen is false", () => {
    render(
      <TestWrapper>
        <ChoiceModal
          isOpen={false}
          onClose={vi.fn()}
          title="Änderung anwenden"
          options={options}
          onSelect={vi.fn()}
        />
      </TestWrapper>,
    );

    expect(screen.queryByText("Änderung anwenden")).not.toBeInTheDocument();
  });

  it("renders title, description and all options", async () => {
    render(
      <TestWrapper>
        <ChoiceModal
          isOpen
          onClose={vi.fn()}
          title="Änderung anwenden"
          description="Wofür soll die Änderung gelten?"
          options={options}
          onSelect={vi.fn()}
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByText("Änderung anwenden")).toBeInTheDocument();
    expect(
      screen.getByText("Wofür soll die Änderung gelten?"),
    ).toBeInTheDocument();
    expect(screen.getByRole("group")).toBeInTheDocument();
    expect(screen.getByText("Nur diesen Termin")).toBeInTheDocument();
    expect(
      screen.getByText("Andere Termine bleiben unverändert"),
    ).toBeInTheDocument();
    expect(screen.getByText("Diesen und alle folgenden")).toBeInTheDocument();
    expect(screen.getByText("Gilt ab diesem Datum")).toBeInTheDocument();
  });

  it("omits the description paragraph when not provided", async () => {
    render(
      <TestWrapper>
        <ChoiceModal
          isOpen
          onClose={vi.fn()}
          title="Änderung anwenden"
          options={options}
          onSelect={vi.fn()}
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(
      screen.queryByText("Wofür soll die Änderung gelten?"),
    ).not.toBeInTheDocument();
  });

  it("calls onSelect with the option value", async () => {
    const onSelect = vi.fn();
    render(
      <TestWrapper>
        <ChoiceModal
          isOpen
          onClose={vi.fn()}
          title="Änderung anwenden"
          options={options}
          onSelect={onSelect}
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(
      screen.getByRole("button", { name: /Diesen und alle folgenden/ }),
    );

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith("this_and_following");
  });

  it("calls onClose when Abbrechen is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <ChoiceModal
          isOpen
          onClose={onClose}
          title="Änderung anwenden"
          options={options}
          onSelect={vi.fn()}
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("disables all options and Abbrechen while isBusy", async () => {
    const onSelect = vi.fn();
    render(
      <TestWrapper>
        <ChoiceModal
          isOpen
          onClose={vi.fn()}
          title="Änderung anwenden"
          options={options}
          onSelect={onSelect}
          isBusy
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const optionButton = screen.getByRole("button", {
      name: /Nur diesen Termin/,
    });
    expect(optionButton).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /Diesen und alle folgenden/ }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();

    fireEvent.click(optionButton);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("ignores backdrop, Escape and close button while isBusy", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <ChoiceModal
          isOpen
          onClose={onClose}
          title="Änderung anwenden"
          options={options}
          onSelect={vi.fn()}
          isBusy
        />
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // The shared Modal defers its onClose by 250ms (exit animation);
    // advance past that after every dismiss attempt.
    fireEvent.keyDown(document, { key: "Escape" });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Hintergrund - Klicken zum Schließen",
      }),
    );
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).not.toHaveBeenCalled();
  });
});
