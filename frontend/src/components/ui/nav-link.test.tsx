import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";

import { ButtonLink } from "./button";
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

describe.each([
  { name: "NavLink", Link: NavLink },
  { name: "ButtonLink", Link: ButtonLink },
])("$name", ({ Link: NavLink }) => {
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

  it.each(["touch", "pen"])(
    "prefetches at once when %s starts on the link",
    (pointerType) => {
      render(<NavLink href="/rooms">Räume</NavLink>);

      fireEvent.pointerEnter(screen.getByRole("link", { name: "Räume" }), {
        pointerType,
      });

      expect(currentPrefetch()).toBe(true);
    },
  );

  it("prefetches when the link receives keyboard focus", () => {
    render(<NavLink href="/staff">Mitarbeitende</NavLink>);

    fireEvent.focus(screen.getByRole("link", { name: "Mitarbeitende" }));

    expect(linkProps).toHaveBeenLastCalledWith({
      href: "/staff",
      prefetch: true,
    });
  });

  it.each(["focus", "hover"])(
    "requires fresh intent after href changes following %s",
    (intent) => {
      vi.useFakeTimers();
      const { rerender } = render(<NavLink href="/rooms">Ziel</NavLink>);
      const link = screen.getByRole("link", { name: "Ziel" });
      if (intent === "focus") {
        fireEvent.focus(link);
      } else {
        fireEvent.pointerEnter(link, { pointerType: "mouse" });
        act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS));
      }
      expect(currentPrefetch()).toBe(true);

      linkProps.mockClear();
      rerender(<NavLink href="/staff">Ziel</NavLink>);
      expect(link).toHaveAttribute("href", "/staff");
      expect(
        linkProps.mock.calls.every(([props]) => props.prefetch === false),
      ).toBe(true);

      rerender(<NavLink href="/rooms">Ziel</NavLink>);
      expect(currentPrefetch()).toBe(false);
      fireEvent.focus(link);
      expect(currentPrefetch()).toBe(true);
    },
  );

  it("cancels a pending hover when href changes", () => {
    vi.useFakeTimers();
    const { rerender } = render(<NavLink href="/rooms">Ziel</NavLink>);
    const link = screen.getByRole("link", { name: "Ziel" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS / 2));

    rerender(<NavLink href="/staff">Ziel</NavLink>);
    expect(link).toHaveAttribute("href", "/staff");
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS));
    expect(currentPrefetch()).toBe(false);

    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS - 1));
    expect(currentPrefetch()).toBe(false);
    act(() => vi.advanceTimersByTime(1));
    expect(currentPrefetch()).toBe(true);
  });

  it("resets intent and cancels pending hover when target changes", () => {
    vi.useFakeTimers();
    const { rerender } = render(<NavLink href="/rooms">Ziel</NavLink>);
    const link = screen.getByRole("link", { name: "Ziel" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    rerender(
      <NavLink href="/rooms" target="_blank">
        Ziel
      </NavLink>,
    );
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS));
    expect(currentPrefetch()).toBe(false);

    rerender(<NavLink href="/rooms">Ziel</NavLink>);
    fireEvent.focus(link);
    expect(currentPrefetch()).toBe(true);
    rerender(
      <NavLink href="/rooms" target="_blank">
        Ziel
      </NavLink>,
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(currentPrefetch()).toBe(false);
    rerender(<NavLink href="/rooms">Ziel</NavLink>);
    expect(currentPrefetch()).toBe(false);
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
