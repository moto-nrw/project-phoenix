import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ParentAttentionBadge } from "./parent-attention-badge";
import { ParentAttentionOverlayProvider } from "~/lib/parent-attention-overlay";
import ParentAttentionOverlayPage from "~/app/[tenant]/(protected)/@parentAttention/(.)parent-attention/[studentId]/page";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  back: vi.fn(),
  replace: vi.fn(),
  scope: "school-a:staff-a",
  pathname: "/parent-attention/7",
  attention: { messages: 2, requests: 1, total: 3 } as {
    messages: number;
    requests: number;
    total: number;
  } | null,
}));
vi.mock("~/lib/tenant-router", () => ({ useTenantRouter: () => mocks }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ studentId: "7" }),
  usePathname: () => mocks.pathname,
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));
vi.mock("~/lib/swr", () => ({
  useSWRCacheKey: () => (key: string) => `${mocks.scope}:${key}`,
}));
vi.mock("~/lib/hooks/use-parent-attention", () => ({
  useStudentParentAttention: () => mocks.attention,
}));
vi.mock("./parent-attention-dialog", () => ({
  ParentAttentionDialog: ({
    studentName,
    attention,
    onClose,
  }: {
    studentName: string;
    attention: { total: number };
    onClose: () => void;
  }) => (
    <div role="dialog" aria-label={studentName}>
      <span>{attention.total} new items</span>
      <button onClick={onClose}>Close</button>
    </div>
  ),
}));

function view(overlay = false) {
  return (
    <ParentAttentionOverlayProvider>
      <ParentAttentionBadge studentId="7" studentName="Mia Kurz" />
      {overlay ? <ParentAttentionOverlayPage /> : null}
    </ParentAttentionOverlayProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.scope = "school-a:staff-a";
  mocks.pathname = "/parent-attention/7";
  mocks.attention = { messages: 2, requests: 1, total: 3 };
});

describe("parent attention route overlay", () => {
  it("opens the selected child in place, keeps the opening counts and closes with Back", () => {
    const { rerender } = render(view());
    fireEvent.click(
      screen.getByRole("button", { name: /der Eltern von Mia Kurz öffnen/ }),
    );
    expect(mocks.push).toHaveBeenCalledWith("/parent-attention/7", {
      scroll: false,
    });
    mocks.attention = null;
    rerender(view(true));
    expect(
      screen.getByRole("dialog", { name: "Mia Kurz" }),
    ).toBeInTheDocument();
    expect(screen.getByText("3 new items")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(mocks.back).toHaveBeenCalledOnce();
  });

  it("closes the retained slot when answering and reopens it on browser Back", () => {
    const { rerender } = render(view());
    fireEvent.click(
      screen.getByRole("button", { name: /der Eltern von Mia Kurz öffnen/ }),
    );
    rerender(view(true));
    expect(
      screen.getByRole("dialog", { name: "Mia Kurz" }),
    ).toBeInTheDocument();
    mocks.pathname = "/messages/8";
    rerender(view(true));
    expect(mocks.replace).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    mocks.pathname = "/parent-attention/7";
    rerender(view(true));
    expect(
      screen.getByRole("dialog", { name: "Mia Kurz" }),
    ).toBeInTheDocument();
  });

  it("does not reuse the selection after the school or account scope changes", () => {
    const { rerender } = render(view());
    fireEvent.click(
      screen.getByRole("button", { name: /der Eltern von Mia Kurz öffnen/ }),
    );
    rerender(view(true));
    expect(
      screen.getByRole("dialog", { name: "Mia Kurz" }),
    ).toBeInTheDocument();
    mocks.scope = "school-b:staff-b";
    rerender(view(true));
    expect(mocks.replace).toHaveBeenCalledWith("/students/7");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
