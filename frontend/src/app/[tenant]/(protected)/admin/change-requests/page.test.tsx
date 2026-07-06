import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import AdminChangeRequestsPage from "./page";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: vi.fn(),
}));

vi.mock("~/components/students/master-data-review-list", () => ({
  MasterDataReviewList: () => <div>master-data-review-list</div>,
}));

vi.mock("~/components/students/care-request-review-list", () => ({
  CareRequestReviewList: () => <div>care-request-review-list</div>,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>loading</div>,
}));

const mockUseRequirePermission = vi.mocked(useRequirePermission);

describe("AdminChangeRequestsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a loading state until the permission check is ready", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: false,
      isLoading: true,
    });

    render(<AdminChangeRequestsPage />);

    expect(screen.getByText("loading")).toBeInTheDocument();
  });

  it("renders both review queues once access is ready", () => {
    mockUseRequirePermission.mockReturnValue({
      isReady: true,
      isLoading: false,
    });

    render(<AdminChangeRequestsPage />);

    expect(
      screen.getByRole("heading", { name: "Änderungsanfragen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("master-data-review-list")).toBeInTheDocument();
    expect(screen.getByText("care-request-review-list")).toBeInTheDocument();
  });
});
