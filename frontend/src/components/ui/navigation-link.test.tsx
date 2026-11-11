import { describe, expect, it, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";

import { ButtonLink } from "./button";
import NavigationLink from "./navigation-link";
import { HOVER_PREFETCH_DELAY_MS } from "./use-link-intent-prefetch";

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
  { name: "NavigationLink", Link: NavigationLink },
  { name: "ButtonLink", Link: ButtonLink },
])("$name", ({ Link }) => {
  beforeEach(() => {
    linkProps.mockClear();
  });

  it("switches the viewport prefetch of next/link off", () => {
    render(<Link href="/rooms">Räume</Link>);

    expect(screen.getByRole("link", { name: "Räume" })).toHaveAttribute(
      "href",
      "/rooms",
    );
    expect(linkProps).toHaveBeenCalledWith({ href: "/rooms", prefetch: false });
    expect(currentPrefetch()).toBe(false);
  });

  it("prefetches the full page once the mouse rests on the link", () => {
    vi.useFakeTimers();
    render(<Link href="/rooms">Räume</Link>);

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
    render(<Link href="/rooms">Räume</Link>);

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
      render(<Link href="/rooms">Räume</Link>);

      fireEvent.pointerEnter(screen.getByRole("link", { name: "Räume" }), {
        pointerType,
      });

      expect(currentPrefetch()).toBe(true);
    },
  );

  it("prefetches when the link receives keyboard focus", () => {
    render(<Link href="/staff">Mitarbeitende</Link>);

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
      const { rerender } = render(<Link href="/rooms">Ziel</Link>);
      const link = screen.getByRole("link", { name: "Ziel" });
      if (intent === "focus") {
        fireEvent.focus(link);
      } else {
        fireEvent.pointerEnter(link, { pointerType: "mouse" });
        act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS));
      }
      expect(currentPrefetch()).toBe(true);

      linkProps.mockClear();
      rerender(<Link href="/staff">Ziel</Link>);
      expect(link).toHaveAttribute("href", "/staff");
      expect(
        linkProps.mock.calls.every(([props]) => props.prefetch === false),
      ).toBe(true);

      rerender(<Link href="/rooms">Ziel</Link>);
      expect(currentPrefetch()).toBe(false);
      fireEvent.focus(link);
      expect(currentPrefetch()).toBe(true);
    },
  );

  it("cancels a pending hover when href changes", () => {
    vi.useFakeTimers();
    const { rerender } = render(<Link href="/rooms">Ziel</Link>);
    const link = screen.getByRole("link", { name: "Ziel" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS / 2));

    rerender(<Link href="/staff">Ziel</Link>);
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
    const { rerender } = render(<Link href="/rooms">Ziel</Link>);
    const link = screen.getByRole("link", { name: "Ziel" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    rerender(
      <Link href="/rooms" target="_blank">
        Ziel
      </Link>,
    );
    act(() => vi.advanceTimersByTime(HOVER_PREFETCH_DELAY_MS));
    expect(currentPrefetch()).toBe(false);

    rerender(<Link href="/rooms">Ziel</Link>);
    fireEvent.focus(link);
    expect(currentPrefetch()).toBe(true);
    rerender(
      <Link href="/rooms" target="_blank">
        Ziel
      </Link>,
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(currentPrefetch()).toBe(false);
    rerender(<Link href="/rooms">Ziel</Link>);
    expect(currentPrefetch()).toBe(false);
  });

  it("keeps the caller's pointer and focus handlers", () => {
    const onPointerEnter = vi.fn();
    const onPointerLeave = vi.fn();
    const onFocus = vi.fn();
    render(
      <Link
        href="/rooms"
        onPointerEnter={onPointerEnter}
        onPointerLeave={onPointerLeave}
        onFocus={onFocus}
      >
        Räume
      </Link>,
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
      <Link href="/help" target="_blank" rel="noopener noreferrer">
        Hilfe
      </Link>,
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
        <Link href="https://example.org/extern">Extern</Link>
        <Link href="//example.org/extern">Protokollrelativ</Link>
        <Link href="mailto:info@example.org">Mail</Link>
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
      <Link
        href="/rooms"
        className="nav-item"
        aria-label="Räume öffnen"
        onClick={onClick}
      >
        Räume
      </Link>,
    );

    const link = screen.getByRole("link", { name: "Räume öffnen" });
    expect(link).toHaveClass("nav-item");
    fireEvent.click(link);
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
