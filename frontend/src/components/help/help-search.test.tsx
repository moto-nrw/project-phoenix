import {
  render,
  screen,
  fireEvent,
  within,
  renderHook,
  act,
  createEvent,
} from "@testing-library/react";
import { afterEach, describe, it, expect, vi } from "vitest";
import { HelpHashScroll, HelpSearchInline, useHelpSearch } from "./help-search";

const deferredValueMock = vi.hoisted(() => ({
  value: null as string | null,
}));

vi.mock("react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react")>();
  return {
    ...actual,
    useDeferredValue: <T,>(value: T) =>
      deferredValueMock.value === null ? value : (deferredValueMock.value as T),
  };
});

// next/link -> plain anchor so result rows render as <a href>.
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...rest
  }: {
    children: React.ReactNode;
    href: string;
  }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

afterEach(() => {
  deferredValueMock.value = null;
  window.location.hash = "";
  vi.useRealTimers();
  vi.restoreAllMocks();
});

function typeQuery(value: string) {
  const input = screen.getByPlaceholderText(/Anleitung durchsuchen/);
  fireEvent.change(input, { target: { value } });
}

describe("HelpSearchInline", () => {
  it("shows nothing until the query reaches the minimum length", () => {
    render(<HelpSearchInline />);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    typeQuery("r");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("returns matching guide sections as deep links", () => {
    render(<HelpSearchInline />);
    typeQuery("Räume anlegen");
    // Title text is split across <mark> highlight spans, so assert on the
    // option's full text content + href rather than an exact text node.
    const options = within(screen.getByRole("listbox")).getAllByRole("option");
    expect(options.length).toBeGreaterThan(0);
    expect(options[0]).toHaveTextContent("Räume anlegen");
    expect(options[0]).toHaveAttribute("href", "/help/setup#raeume-anlegen");
  });

  it("tolerates typos and missing umlauts (fuzzy match)", () => {
    render(<HelpSearchInline />);
    typeQuery("raum anlegen");
    expect(screen.getAllByRole("option")[0]).toHaveTextContent("Räume anlegen");
  });

  it("matches multi-word queries token-by-token (Kind finden -> Alle Kinder)", () => {
    render(<HelpSearchInline />);
    typeQuery("Kind finden");
    const firstOption = screen.getAllByRole("option")[0];
    expect(firstOption).toHaveTextContent("Alle Kinder");
    expect(firstOption?.closest("a")).toHaveAttribute(
      "href",
      "/help/features#kindersuche",
    );
  });

  it("highlights only the contiguous typed substring, never scattered letters", () => {
    render(<HelpSearchInline />);
    typeQuery("kind");
    const option = screen
      .getAllByRole("option")
      .find((o) => o.textContent?.includes("Alle Kinder"));
    expect(option).toBeDefined();
    const marks = option!.querySelectorAll("mark");
    // Every mark is the clean contiguous token "Kind" — both the title and the
    // summary ("…jedes Kind…") highlight it, but never as jagged per-fuzzy-char
    // marks. The number of marks depends on how often the token appears in the
    // displayed fields; the invariant under test is that each one is clean.
    expect(marks.length).toBeGreaterThan(0);
    for (const mark of marks) {
      expect(mark).toHaveTextContent(/^Kind$/i);
    }
  });

  it("shows an empty state when nothing matches", () => {
    render(<HelpSearchInline />);
    typeQuery("zzzqqxx");
    expect(screen.getByText(/Keine Treffer/)).toBeInTheDocument();
  });

  it("does not trap listbox keys after results are dismissed", () => {
    render(<HelpSearchInline />);
    typeQuery("Räume anlegen");

    const input = screen.getByPlaceholderText(/Anleitung durchsuchen/);
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.keyDown(input, { key: "Escape" });
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();

    for (const key of ["ArrowDown", "Home", "End", "Enter"]) {
      const event = createEvent.keyDown(input, { key });
      fireEvent(input, event);
      expect(event.defaultPrevented).toBe(false);
    }
  });

  it("hides stale deferred results once the current query is below the minimum", () => {
    deferredValueMock.value = "Räume anlegen";
    render(<HelpSearchInline />);

    const input = screen.getByPlaceholderText(/Anleitung durchsuchen/);
    fireEvent.change(input, { target: { value: "Räume anlegen" } });
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.change(input, { target: { value: "r" } });
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(input).toHaveAttribute("aria-expanded", "false");

    const enter = createEvent.keyDown(input, { key: "Enter" });
    fireEvent(input, enter);
    expect(enter.defaultPrevented).toBe(false);
  });

  it("hides stale deferred results while a new valid query is pending", () => {
    deferredValueMock.value = "Räume anlegen";
    render(<HelpSearchInline />);

    const input = screen.getByPlaceholderText(/Anleitung durchsuchen/);
    fireEvent.change(input, { target: { value: "Räume anlegen" } });
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.change(input, { target: { value: "Alle Kinder" } });
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(screen.queryByText(/Keine Treffer/)).not.toBeInTheDocument();
    expect(input).toHaveAttribute("aria-expanded", "false");

    const enter = createEvent.keyDown(input, { key: "Enter" });
    fireEvent(input, enter);
    expect(enter.defaultPrevented).toBe(false);
  });

  it("highlights matches in the summary line, not only the title", () => {
    render(<HelpSearchInline />);
    typeQuery("kind");
    const option = screen
      .getAllByRole("option")
      .find((o) => o.textContent?.includes("Alle Kinder"));
    // "Kind" appears in both the title ("Kindersuche") and the summary
    // ("…jedes Kind…"), so a summary-aware highlighter yields ≥2 marks.
    expect(option!.querySelectorAll("mark").length).toBeGreaterThanOrEqual(2);
  });
  it("walks the result list with ArrowDown/ArrowUp and wraps around", () => {
    const scrollSpy = vi
      .spyOn(HTMLElement.prototype, "scrollIntoView")
      .mockImplementation(() => {});
    render(<HelpSearchInline />);
    typeQuery("anlegen"); // several guide cards contain "anlegen"

    const input = screen.getByPlaceholderText(/Anleitung durchsuchen/);
    const optionCount = screen.getAllByRole("option").length;
    expect(optionCount).toBeGreaterThan(1);

    const activeId = () =>
      screen
        .getAllByRole("option")
        .find((o) => o.getAttribute("aria-selected") === "true")?.id;

    // Selection starts on the first row and the input advertises it.
    expect(screen.getAllByRole("option")[0]).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(input).toHaveAttribute("aria-activedescendant", activeId());

    const down = createEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent(input, down);
    expect(down.defaultPrevented).toBe(true);
    expect(screen.getAllByRole("option")[1]).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(input).toHaveAttribute("aria-activedescendant", activeId());

    // ArrowUp from the first row wraps to the last.
    fireEvent.keyDown(input, { key: "ArrowUp" });
    fireEvent.keyDown(input, { key: "ArrowUp" });
    expect(screen.getAllByRole("option")[optionCount - 1]).toHaveAttribute(
      "aria-selected",
      "true",
    );
    scrollSpy.mockRestore();
  });

  it("jumps to the first and last row with Home/End", () => {
    const scrollSpy = vi
      .spyOn(HTMLElement.prototype, "scrollIntoView")
      .mockImplementation(() => {});
    render(<HelpSearchInline />);
    typeQuery("anlegen");

    const input = screen.getByPlaceholderText(/Anleitung durchsuchen/);
    const optionCount = screen.getAllByRole("option").length;

    const end = createEvent.keyDown(input, { key: "End" });
    fireEvent(input, end);
    expect(end.defaultPrevented).toBe(true);
    expect(screen.getAllByRole("option")[optionCount - 1]).toHaveAttribute(
      "aria-selected",
      "true",
    );

    const home = createEvent.keyDown(input, { key: "Home" });
    fireEvent(input, home);
    expect(home.defaultPrevented).toBe(true);
    expect(screen.getAllByRole("option")[0]).toHaveAttribute(
      "aria-selected",
      "true",
    );
    scrollSpy.mockRestore();
  });

  it("opens the active row and clears the query when Enter is pressed", () => {
    const scrollSpy = vi
      .spyOn(HTMLElement.prototype, "scrollIntoView")
      .mockImplementation(() => {});
    // Stub the anchor click so happy-dom doesn't attempt real navigation;
    // the row's onClick (which clears the query) still fires through it.
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(function (this: HTMLAnchorElement) {
        this.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      });
    render(<HelpSearchInline />);
    typeQuery("Räume anlegen");

    const input = screen.getByPlaceholderText(/Anleitung durchsuchen/);
    const enter = createEvent.keyDown(input, { key: "Enter" });
    fireEvent(input, enter);

    expect(enter.defaultPrevented).toBe(true);
    expect(clickSpy).toHaveBeenCalled();
    // onNavigate cleared the query, so the listbox collapses.
    expect(input).toHaveValue("");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    clickSpy.mockRestore();
    scrollSpy.mockRestore();
  });

  it("activates a row on mouse enter", () => {
    render(<HelpSearchInline />);
    typeQuery("anlegen");
    const options = screen.getAllByRole("option");
    expect(options.length).toBeGreaterThan(1);

    fireEvent.mouseEnter(options[1]!);
    expect(screen.getAllByRole("option")[1]).toHaveAttribute(
      "aria-selected",
      "true",
    );
  });

  it("dismisses the panel on an outside pointer press but keeps the query", () => {
    render(<HelpSearchInline />);
    typeQuery("Räume anlegen");
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.mouseDown(document.body);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    // The typed text is preserved; only the dropdown is dismissed.
    expect(screen.getByPlaceholderText(/Anleitung durchsuchen/)).toHaveValue(
      "Räume anlegen",
    );
  });

  it("coalesces overlapping highlight ranges into a single mark", () => {
    render(<HelpSearchInline />);
    // "kind" (0–3) and "kinder" (0–5) both match "Kindersuche"; their ranges
    // overlap and must merge into one clean <mark> rather than nested spans.
    typeQuery("kind kinder");
    const option = screen
      .getAllByRole("option")
      .find((o) => o.textContent?.includes("Alle Kinder"));
    expect(option).toBeDefined();
    const marks = [...option!.querySelectorAll("mark")];
    expect(marks.some((m) => m.textContent === "Kinder")).toBe(true);
    // No mark is a strict prefix nested inside another (no jagged double-mark).
    expect(marks.some((m) => m.querySelector("mark"))).toBe(false);
  });
});

describe("useHelpSearch", () => {
  it("ranks an exact title match above body-only matches", () => {
    const { result } = renderHook(() => useHelpSearch());
    act(() => result.current.setQuery("Räume anlegen"));
    expect(result.current.hits[0]?.record.title).toBe("Räume anlegen");
    expect(result.current.hits[0]?.titleRanges.length).toBeGreaterThan(0);
  });

  it("matches across the ß/ss boundary (schliessen -> schließen)", () => {
    const { result } = renderHook(() => useHelpSearch());
    act(() => result.current.setQuery("schliessen"));
    expect(result.current.hits.length).toBeGreaterThan(0);
  });

  it("exposes summary highlight ranges on hits", () => {
    const { result } = renderHook(() => useHelpSearch());
    act(() => result.current.setQuery("Alle Kinder"));
    const top = result.current.hits[0];
    expect(top?.record.title).toBe("Alle Kinder");
    expect(top?.summaryRanges).toBeDefined();
  });

  it("keeps the former Kindersuche name as a search alias", () => {
    const { result } = renderHook(() => useHelpSearch());
    act(() => result.current.setQuery("Kindersuche"));
    expect(result.current.hits[0]?.record.title).toBe("Alle Kinder");
  });
});

describe("HelpHashScroll", () => {
  it("schedules delayed re-scrolls after same-page hash changes", () => {
    vi.useFakeTimers();
    const target = document.createElement("div");
    target.id = "target";
    target.scrollIntoView = vi.fn();
    document.body.appendChild(target);
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      callback(0);
      return 1;
    });

    render(<HelpHashScroll />);

    act(() => {
      window.location.hash = "#target";
      window.dispatchEvent(new Event("hashchange"));
    });

    expect(target.scrollIntoView).toHaveBeenCalledTimes(2);

    act(() => vi.advanceTimersByTime(150));
    expect(target.scrollIntoView).toHaveBeenCalledTimes(3);

    act(() => vi.advanceTimersByTime(450));
    expect(target.scrollIntoView).toHaveBeenCalledTimes(4);
  });

  it("re-scrolls when a lazy image above the hash target loads late", () => {
    vi.useFakeTimers();
    const imageAbove = document.createElement("img");
    const target = document.createElement("div");
    const imageBelow = document.createElement("img");
    target.id = "late-image-target";
    target.scrollIntoView = vi.fn();
    document.body.append(imageAbove, target, imageBelow);
    vi.spyOn(window, "requestAnimationFrame").mockImplementation((callback) => {
      callback(0);
      return 1;
    });

    render(<HelpHashScroll />);

    act(() => {
      window.location.hash = "#late-image-target";
      window.dispatchEvent(new Event("hashchange"));
    });

    expect(target.scrollIntoView).toHaveBeenCalledTimes(2);

    act(() => vi.advanceTimersByTime(600));
    expect(target.scrollIntoView).toHaveBeenCalledTimes(4);

    act(() => {
      imageAbove.dispatchEvent(new Event("load"));
    });
    expect(target.scrollIntoView).toHaveBeenCalledTimes(5);

    act(() => {
      imageBelow.dispatchEvent(new Event("load"));
    });
    expect(target.scrollIntoView).toHaveBeenCalledTimes(5);
  });

  it("does nothing when there is no hash on mount", () => {
    vi.useFakeTimers();
    const rafSpy = vi
      .spyOn(window, "requestAnimationFrame")
      .mockImplementation((callback) => {
        callback(0);
        return 1;
      });
    const scrollSpy = vi
      .spyOn(HTMLElement.prototype, "scrollIntoView")
      .mockImplementation(() => {});

    render(<HelpHashScroll />);
    act(() => vi.advanceTimersByTime(600));

    // No hash -> scheduleHashScrolls bails before scheduling any scroll.
    expect(scrollSpy).not.toHaveBeenCalled();
    rafSpy.mockRestore();
    scrollSpy.mockRestore();
  });

  it("ignores load events from non-image elements and when no hash target exists", () => {
    const scrollSpy = vi
      .spyOn(HTMLElement.prototype, "scrollIntoView")
      .mockImplementation(() => {});
    const orphanImage = document.createElement("img");
    const notAnImage = document.createElement("div");
    document.body.append(orphanImage, notAnImage);

    render(<HelpHashScroll />);

    act(() => {
      // Non-image target -> handler returns immediately.
      notAnImage.dispatchEvent(new Event("load"));
      // Image load but no hash set -> getHashTarget() is null, returns early.
      orphanImage.dispatchEvent(new Event("load"));
    });

    expect(scrollSpy).not.toHaveBeenCalled();
    scrollSpy.mockRestore();
  });
});
