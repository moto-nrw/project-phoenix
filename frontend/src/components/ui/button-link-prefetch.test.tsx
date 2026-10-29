import { act, fireEvent, render, screen } from "@testing-library/react";
import { createRef, type ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { HomeCardLink } from "../home/home-card-rows";
import { ButtonLink } from "./button";

vi.mock("next/link", () => ({
  default: ({
    prefetch,
    children,
    onNavigate: _onNavigate,
    ...props
  }: ComponentProps<"a"> & {
    prefetch?: boolean | null;
    onNavigate?: unknown;
  }) => (
    <a {...props} data-prefetch={String(prefetch)}>
      {children}
    </a>
  ),
}));

describe("ButtonLink intent prefetch", () => {
  it("prefetches the full route only after sustained mouse intent", () => {
    vi.useFakeTimers();
    render(<ButtonLink href="/ogs-groups">Zur Gruppe</ButtonLink>);
    const link = screen.getByRole("link", { name: "Zur Gruppe" });
    expect(link).toHaveAttribute("data-prefetch", "false");
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    act(() => vi.advanceTimersByTime(79));
    expect(link).toHaveAttribute("data-prefetch", "false");
    act(() => vi.advanceTimersByTime(1));
    expect(link).toHaveAttribute("data-prefetch", "true");
  });
  it("forwards the anchor ref and preserves the caller's prefetch opt-out", () => {
    const ref = createRef<HTMLAnchorElement>();
    render(
      <ButtonLink href="/ogs-groups" ref={ref} prefetch={false}>
        Zur Gruppe
      </ButtonLink>,
    );
    const link = screen.getByRole("link", { name: "Zur Gruppe" });
    expect(ref.current).toBe(link);
    fireEvent.focus(link);
    fireEvent.pointerEnter(link, { pointerType: "pen" });
    expect(link).toHaveAttribute("data-prefetch", "false");
  });

  it("clears a pending hover timer on unmount", () => {
    vi.useFakeTimers();
    const { unmount } = render(
      <ButtonLink href="/ogs-groups">Zur Gruppe</ButtonLink>,
    );
    const link = screen.getByRole("link", { name: "Zur Gruppe" });
    const timersBefore = vi.getTimerCount();
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    expect(vi.getTimerCount()).toBe(timersBefore + 1);
    unmount();
    expect(vi.getTimerCount()).toBe(timersBefore);
  });

  it("prefetches the HomeCardLink destination on focus without changing its accessible name", () => {
    render(
      <HomeCardLink href="/ogs-groups" label="Meine Gruppe öffnen">
        Zur Gruppe
      </HomeCardLink>,
    );
    const link = screen.getByRole("link", { name: "Meine Gruppe öffnen" });
    expect(link).toHaveAttribute("href", "/ogs-groups");
    expect(link).toHaveTextContent("Zur Gruppe");
    expect(link).toHaveAttribute("data-prefetch", "false");
    fireEvent.focus(link);
    expect(link).toHaveAttribute("data-prefetch", "true");
  });
});
