import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";

import { NavLink } from "./nav-link";
import { HOVER_PREFETCH_DELAY_MS } from "./navigation-link";

const linkProps = vi.fn();

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    prefetch,
    ...rest
  }: ComponentProps<"a"> & { prefetch?: boolean | null }) => {
    linkProps({ href, prefetch });
    return (
      <a href={href} {...rest}>
        {children}
      </a>
    );
  },
}));

/** The `prefetch` value next/link received on its latest render. */
function currentPrefetch(): boolean | null | undefined {
  const calls = linkProps.mock.calls;
  return (calls.at(-1)?.[0] as { prefetch?: boolean | null } | undefined)
    ?.prefetch;
}

describe("NavLink", () => {
  beforeEach(() => {
    linkProps.mockClear();
  });

  it("switches the viewport prefetch of next/link off", () => {
    render(<NavLink href="/rooms">Räume</NavLink>);

    expect(screen.getByRole("link", { name: "Räume" })).toHaveAttribute(
      "href",
      "/rooms",
    );
    expect(linkProps).toHaveBeenCalledWith({ href: "/rooms", prefetch: false });
    expect(currentPrefetch()).toBe(false);
  });

  it("prefetches the full page once the mouse rests on the link", () => {
    vi.useFakeTimers();
    render(<NavLink href="/rooms">Räume</NavLink>);

    fireEvent.pointerEnter(screen.getByRole("link", { name: "Räume" }), {
      pointerType: "mouse",
    });
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS - 1));
    expect(currentPrefetch()).toBe(false);

    act(() => vi.advanceTimersByTime(1));
    expect(currentPrefetch()).toBe(true);
  });

  it("does not prefetch when the mouse only passes over the link", () => {
    vi.useFakeTimers();
    render(<NavLink href="/rooms">Räume</NavLink>);

    const link = screen.getByRole("link", { name: "Räume" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS / 2));
    fireEvent.pointerLeave(link, { pointerType: "mouse" });
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS * 2));

    expect(currentPrefetch()).toBe(false);
  });

  it("prefetches at once when a touch starts on the link", () => {
    render(<NavLink href="/rooms">Räume</NavLink>);

    fireEvent.pointerEnter(screen.getByRole("link", { name: "Räume" }), {
      pointerType: "touch",
    });

    expect(currentPrefetch()).toBe(true);
  });

  it("prefetches when the link receives keyboard focus", () => {
    render(<NavLink href="/staff">Mitarbeitende</NavLink>);

    fireEvent.focus(screen.getByRole("link", { name: "Mitarbeitende" }));

    expect(linkProps).toHaveBeenLastCalledWith({
      href: "/staff",
      prefetch: true,
    });
  });

  it("keeps the caller's pointer and focus handlers", () => {
    const onPointerEnter = vi.fn();
    const onPointerLeave = vi.fn();
    const onFocus = vi.fn();
    render(
      <NavLink
        href="/rooms"
        onPointerEnter={onPointerEnter}
        onPointerLeave={onPointerLeave}
        onFocus={onFocus}
      >
        Räume
      </NavLink>,
    );

    const link = screen.getByRole("link", { name: "Räume" });
    fireEvent.pointerEnter(link, { pointerType: "touch" });
    fireEvent.pointerLeave(link, { pointerType: "touch" });
    fireEvent.focus(link);

    expect(onPointerEnter).toHaveBeenCalledTimes(1);
    expect(onPointerLeave).toHaveBeenCalledTimes(1);
    expect(onFocus).toHaveBeenCalledTimes(1);
    expect(currentPrefetch()).toBe(true);
  });

  it("never prefetches links that open a new tab", () => {
    vi.useFakeTimers();
    render(
      <NavLink href="/help" target="_blank" rel="noopener noreferrer">
        Hilfe
      </NavLink>,
    );

    const link = screen.getByRole("link", { name: "Hilfe" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS * 2));
    fireEvent.pointerEnter(link, { pointerType: "touch" });
    fireEvent.focus(link);

    expect(link).toHaveAttribute("target", "_blank");
    expect(currentPrefetch()).toBe(false);
  });

  it("never prefetches hrefs that are not same-origin paths", () => {
    render(
      <>
        <NavLink href="https://example.org/extern">Extern</NavLink>
        <NavLink href="//example.org/extern">Protokollrelativ</NavLink>
        <NavLink href="mailto:info@example.org">Mail</NavLink>
      </>,
    );

    for (const name of ["Extern", "Protokollrelativ", "Mail"]) {
      const link = screen.getByRole("link", { name });
      fireEvent.pointerEnter(link, { pointerType: "touch" });
      fireEvent.focus(link);
    }

    expect(linkProps).not.toHaveBeenCalledWith(
      expect.objectContaining({ prefetch: true }),
    );
  });

  it("passes className, aria and click handlers through to the anchor", () => {
    const onClick = vi.fn();
    render(
      <NavLink
        href="/rooms"
        className="nav-item"
        aria-label="Räume öffnen"
        onClick={onClick}
      >
        Räume
      </NavLink>,
    );

    const link = screen.getByRole("link", { name: "Räume öffnen" });
    expect(link).toHaveClass("nav-item");
    fireEvent.click(link);
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
