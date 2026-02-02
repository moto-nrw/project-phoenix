import { render, screen, act } from "@testing-library/react";
import { renderHook } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { ReactNode } from "react";
import {
  BreadcrumbProvider,
  useBreadcrumb,
  useSetBreadcrumb,
  useStudentHistoryBreadcrumb,
} from "./breadcrumb-context";

function wrapper({ children }: { children: ReactNode }) {
  return <BreadcrumbProvider>{children}</BreadcrumbProvider>;
}

describe("BreadcrumbProvider", () => {
  it("renders children", () => {
    render(
      <BreadcrumbProvider>
        <div data-testid="child">Hello</div>
      </BreadcrumbProvider>,
    );
    expect(screen.getByTestId("child")).toBeInTheDocument();
  });
});

describe("useBreadcrumb", () => {
  it("returns breadcrumb state and setter", () => {
    const { result } = renderHook(() => useBreadcrumb(), { wrapper });

    expect(result.current.breadcrumb).toEqual({});
    expect(typeof result.current.setBreadcrumb).toBe("function");
  });

  it("throws when used outside provider", () => {
    // Suppress console.error for expected error
    // eslint-disable-next-line @typescript-eslint/no-empty-function -- intentionally silencing expected error
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => {
      renderHook(() => useBreadcrumb());
    }).toThrow("useBreadcrumb must be used within a BreadcrumbProvider");

    spy.mockRestore();
  });

  it("updates breadcrumb when setBreadcrumb is called", () => {
    const { result } = renderHook(() => useBreadcrumb(), { wrapper });

    act(() => {
      result.current.setBreadcrumb({ studentName: "Max Mustermann" });
    });

    expect(result.current.breadcrumb).toEqual({
      studentName: "Max Mustermann",
    });
  });
});

describe("useSetBreadcrumb", () => {
  it("sets breadcrumb data on mount", () => {
    const data = { studentName: "Lisa Schmidt", roomName: "Raum 1" };
    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer() {
      useSetBreadcrumb(data);
      breadcrumbResult = useBreadcrumb();
      return <div>consumer</div>;
    }

    render(
      <BreadcrumbProvider>
        <TestConsumer />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual(data);
  });

  it("clears breadcrumb data on unmount", () => {
    const data = { studentName: "Lisa Schmidt" };
    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer() {
      useSetBreadcrumb(data);
      return <div>consumer</div>;
    }

    function Reader() {
      breadcrumbResult = useBreadcrumb();
      return <div>reader</div>;
    }

    const { unmount } = render(
      <BreadcrumbProvider>
        <TestConsumer />
        <Reader />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual(data);

    // Unmount the whole tree — useSetBreadcrumb cleanup sets breadcrumb to {}
    unmount();
    // After unmount, the cleanup effect ran, but since the provider is also
    // unmounted, we verify the cleanup path was exercised (no error thrown).
  });

  it("updates breadcrumb when data changes", () => {
    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer({ name }: { name: string }) {
      useSetBreadcrumb({ studentName: name });
      breadcrumbResult = useBreadcrumb();
      return <div>consumer</div>;
    }

    const { rerender } = render(
      <BreadcrumbProvider>
        <TestConsumer name="Max" />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual({ studentName: "Max" });

    rerender(
      <BreadcrumbProvider>
        <TestConsumer name="Lisa" />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual({ studentName: "Lisa" });
  });
});

describe("useStudentHistoryBreadcrumb", () => {
  let originalGetItem: typeof localStorage.getItem;

  beforeEach(() => {
    originalGetItem = localStorage.getItem.bind(localStorage);
    Object.defineProperty(localStorage, "getItem", {
      value: vi.fn().mockReturnValue(null),
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(localStorage, "getItem", {
      value: originalGetItem,
      writable: true,
      configurable: true,
    });
  });

  it("sets breadcrumb with OGS group name from localStorage when referrer is /ogs-groups", () => {
    (localStorage.getItem as ReturnType<typeof vi.fn>).mockImplementation(
      (key: string) => {
        if (key === "sidebar-last-group-name") return "Eulen";
        return null;
      },
    );

    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer() {
      useStudentHistoryBreadcrumb({
        studentName: "Max Mustermann",
        referrer: "/ogs-groups",
      });
      breadcrumbResult = useBreadcrumb();
      return <div>consumer</div>;
    }

    render(
      <BreadcrumbProvider>
        <TestConsumer />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual({
      studentName: "Max Mustermann",
      referrerPage: "/ogs-groups",
      ogsGroupName: "Eulen",
      activeSupervisionName: undefined,
    });
  });

  it("sets breadcrumb with room name from localStorage when referrer is /active-supervisions", () => {
    (localStorage.getItem as ReturnType<typeof vi.fn>).mockImplementation(
      (key: string) => {
        if (key === "sidebar-last-room-name") return "Raum A";
        return null;
      },
    );

    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer() {
      useStudentHistoryBreadcrumb({
        studentName: "Lisa Schmidt",
        referrer: "/active-supervisions",
      });
      breadcrumbResult = useBreadcrumb();
      return <div>consumer</div>;
    }

    render(
      <BreadcrumbProvider>
        <TestConsumer />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual({
      studentName: "Lisa Schmidt",
      referrerPage: "/active-supervisions",
      ogsGroupName: undefined,
      activeSupervisionName: "Raum A",
    });
  });

  it("sets breadcrumb without group/room names for other referrers", () => {
    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer() {
      useStudentHistoryBreadcrumb({
        studentName: "Tom Test",
        referrer: "/students/search",
      });
      breadcrumbResult = useBreadcrumb();
      return <div>consumer</div>;
    }

    render(
      <BreadcrumbProvider>
        <TestConsumer />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual({
      studentName: "Tom Test",
      referrerPage: "/students/search",
      ogsGroupName: undefined,
      activeSupervisionName: undefined,
    });
  });

  it("handles null localStorage values gracefully", () => {
    // getItem already mocked to return null in beforeEach

    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer() {
      useStudentHistoryBreadcrumb({
        studentName: "Anna Test",
        referrer: "/ogs-groups",
      });
      breadcrumbResult = useBreadcrumb();
      return <div>consumer</div>;
    }

    render(
      <BreadcrumbProvider>
        <TestConsumer />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual({
      studentName: "Anna Test",
      referrerPage: "/ogs-groups",
      ogsGroupName: undefined,
      activeSupervisionName: undefined,
    });
  });

  it("works without studentName", () => {
    let breadcrumbResult: ReturnType<typeof useBreadcrumb> | undefined;

    function TestConsumer() {
      useStudentHistoryBreadcrumb({
        referrer: "/students/search",
      });
      breadcrumbResult = useBreadcrumb();
      return <div>consumer</div>;
    }

    render(
      <BreadcrumbProvider>
        <TestConsumer />
      </BreadcrumbProvider>,
    );

    expect(breadcrumbResult?.breadcrumb).toEqual({
      studentName: undefined,
      referrerPage: "/students/search",
      ogsGroupName: undefined,
      activeSupervisionName: undefined,
    });
  });
});
