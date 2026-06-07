import { describe, it, expect, vi, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

import { useClickOutside } from "./use-click-outside";

function mountContainer() {
  const container = document.createElement("div");
  const child = document.createElement("button");
  container.appendChild(child);
  document.body.appendChild(container);
  return { container, child };
}

describe("useClickOutside", () => {
  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("calls onDismiss on a mousedown outside the container", () => {
    const { container } = mountContainer();
    const outside = document.createElement("div");
    document.body.appendChild(outside);
    const onDismiss = vi.fn();

    renderHook(() => useClickOutside({ current: container }, onDismiss, true));

    outside.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("does not call onDismiss on a mousedown inside the container", () => {
    const { container, child } = mountContainer();
    const onDismiss = vi.fn();

    renderHook(() => useClickOutside({ current: container }, onDismiss, true));

    child.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("calls onDismiss when Escape is pressed", () => {
    const { container } = mountContainer();
    const onDismiss = vi.fn();

    renderHook(() => useClickOutside({ current: container }, onDismiss, true));

    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("does not attach listeners when disabled", () => {
    const { container } = mountContainer();
    const outside = document.createElement("div");
    document.body.appendChild(outside);
    const onDismiss = vi.fn();

    renderHook(() => useClickOutside({ current: container }, onDismiss, false));

    outside.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" }));
    expect(onDismiss).not.toHaveBeenCalled();
  });
});
