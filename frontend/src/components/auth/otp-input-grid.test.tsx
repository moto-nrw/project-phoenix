import { createRef } from "react";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import { OTPInputGrid, type OTPInputGridHandle } from "./otp-input-grid";

function renderGrid(props: {
  onComplete: (code: string) => void;
  length?: number;
  disabled?: boolean;
  autoFocus?: boolean;
  ariaLabel?: string;
}) {
  const ref = createRef<OTPInputGridHandle>();
  const utils = render(
    <OTPInputGrid
      ref={ref}
      length={props.length ?? 6}
      disabled={props.disabled}
      autoFocus={props.autoFocus}
      onComplete={props.onComplete}
      ariaLabel={props.ariaLabel}
    />,
  );
  const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
  return { ...utils, ref, inputs };
}

describe("OTPInputGrid", () => {
  it("renders the configured number of input slots", () => {
    renderGrid({ length: 6, onComplete: vi.fn() });
    expect(screen.getAllByRole("textbox")).toHaveLength(6);
  });

  it("uses a custom aria-label on the container when provided", () => {
    renderGrid({ onComplete: vi.fn(), ariaLabel: "6-stelliger Code" });
    expect(screen.getByLabelText("6-stelliger Code")).toBeInTheDocument();
  });

  it("fires onComplete exactly once when all digits are filled", async () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });

    // Typing into each slot in sequence — auto-advance is handled by the
    // requestAnimationFrame focus inside the component.
    for (let i = 0; i < 6; i++) {
      fireEvent.change(inputs[i]!, { target: { value: String(i + 1) } });
    }

    expect(onComplete).toHaveBeenCalledTimes(1);
    expect(onComplete).toHaveBeenCalledWith("123456");
  });

  it("does not fire onComplete twice when extra input arrives after a fill", async () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });

    for (let i = 0; i < 6; i++) {
      fireEvent.change(inputs[i]!, { target: { value: String(i + 1) } });
    }
    expect(onComplete).toHaveBeenCalledTimes(1);

    // Subsequent edits without a reset must NOT re-fire — that would
    // cause double-submit on the parent's verify flow.
    fireEvent.change(inputs[5]!, { target: { value: "9" } });
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("strips non-digit characters from inputs", () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });
    fireEvent.change(inputs[0]!, { target: { value: "a" } });
    expect(inputs[0]!.value).toBe("");
  });

  it("distributes a pasted 6-digit code across all slots", async () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });

    // Simulate paste of "987654"
    fireEvent.paste(inputs[0]!, {
      clipboardData: {
        getData: () => "987654",
      },
    });

    expect(onComplete).toHaveBeenCalledWith("987654");
  });

  it("ignores pastes with no digits and leaves state untouched", () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });

    fireEvent.paste(inputs[0]!, {
      clipboardData: { getData: () => "no digits here" },
    });

    expect(onComplete).not.toHaveBeenCalled();
    inputs.forEach((i) => expect(i.value).toBe(""));
  });

  it("Backspace on a filled slot clears that slot", () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });

    fireEvent.change(inputs[0]!, { target: { value: "1" } });
    expect(inputs[0]!.value).toBe("1");

    inputs[0]!.focus();
    fireEvent.keyDown(inputs[0]!, { key: "Backspace" });
    expect(inputs[0]!.value).toBe("");
  });

  it("Backspace on an empty slot moves focus left and clears the previous slot", () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });

    fireEvent.change(inputs[0]!, { target: { value: "5" } });
    inputs[1]!.focus();
    fireEvent.keyDown(inputs[1]!, { key: "Backspace" });

    expect(document.activeElement).toBe(inputs[0]!);
    expect(inputs[0]!.value).toBe("");
  });

  it("ArrowLeft/Right move focus between slots", () => {
    const onComplete = vi.fn();
    const { inputs } = renderGrid({ onComplete });

    inputs[2]!.focus();
    fireEvent.keyDown(inputs[2]!, { key: "ArrowLeft" });
    expect(document.activeElement).toBe(inputs[1]!);
    fireEvent.keyDown(inputs[1]!, { key: "ArrowRight" });
    fireEvent.keyDown(inputs[2]!, { key: "ArrowRight" });
    expect(document.activeElement).toBe(inputs[3]!);
  });

  it("disabled prop locks every input", () => {
    const { inputs } = renderGrid({ onComplete: vi.fn(), disabled: true });
    inputs.forEach((i) => expect(i).toBeDisabled());
  });

  it("imperative reset() clears all slots and refocuses the first", async () => {
    const onComplete = vi.fn();
    const { inputs, ref } = renderGrid({ onComplete });

    for (let i = 0; i < 6; i++) {
      fireEvent.change(inputs[i]!, { target: { value: String(i + 1) } });
    }
    expect(onComplete).toHaveBeenCalledTimes(1);

    act(() => {
      ref.current?.reset();
    });

    inputs.forEach((i) => expect(i.value).toBe(""));
    expect(document.activeElement).toBe(inputs[0]!);

    // After reset the submit guard releases — a fresh fill must trigger
    // onComplete again.
    for (let i = 0; i < 6; i++) {
      fireEvent.change(inputs[i]!, { target: { value: String(9 - i) } });
    }
    expect(onComplete).toHaveBeenCalledTimes(2);
    expect(onComplete).toHaveBeenLastCalledWith("987654");
  });

  it("imperative focus() puts focus on the first slot", () => {
    const onComplete = vi.fn();
    const { inputs, ref } = renderGrid({ onComplete, autoFocus: false });

    // Detach focus first to verify focus() actually moves it.
    document.body.focus();
    expect(document.activeElement).not.toBe(inputs[0]!);

    act(() => {
      ref.current?.focus();
    });
    expect(document.activeElement).toBe(inputs[0]!);
  });
});
