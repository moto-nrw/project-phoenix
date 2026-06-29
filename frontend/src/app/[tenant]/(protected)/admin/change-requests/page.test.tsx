import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AdminChangeRequestsPage from "./page";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

vi.mock("~/lib/hooks/use-require-admin", () => ({
  useRequireAdmin: vi.fn(),
}));

vi.mock("~/components/students/master-data-review-list", () => ({
  MasterDataReviewList: () => <div>review-list</div>,
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({ title }: { title: string }) => <h1>{title}</h1>,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>loading</div>,
}));

vi.mock("~/components/ui/desktop-only-notice", () => ({
  DesktopOnlyNotice: () => <div>desktop-notice</div>,
}));

const mockUseRequireAdmin = vi.mocked(useRequireAdmin);

describe("AdminChangeRequestsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a loading state until admin access is ready", () => {
    mockUseRequireAdmin.mockReturnValue({ isReady: false, isLoading: true });

    render(<AdminChangeRequestsPage />);

    expect(screen.getByText("loading")).toBeInTheDocument();
  });

  it("renders the review queue once admin access is ready", () => {
    mockUseRequireAdmin.mockReturnValue({ isReady: true, isLoading: false });

    render(<AdminChangeRequestsPage />);

    expect(
      screen.getByRole("heading", { name: "Änderungsanfragen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("desktop-notice")).toBeInTheDocument();
    expect(screen.getByText("review-list")).toBeInTheDocument();
  });
});
