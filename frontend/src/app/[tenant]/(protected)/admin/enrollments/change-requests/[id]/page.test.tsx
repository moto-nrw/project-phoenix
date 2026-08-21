import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import Page from "./page";
import { canReviewEnrollmentChangeRequests } from "~/lib/change-request-access";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";

vi.mock("~/components/enrollment/admin-enrollment-change-requests", () => ({
  AdminEnrollmentChangeRequestDetail: ({
    changeRequestId,
  }: {
    changeRequestId: string;
  }) => <div>Detail {changeRequestId}</div>,
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: vi.fn(() => ({ isReady: true, isLoading: false })),
}));

describe("AdminEnrollmentChangeRequestDetailPage", () => {
  it("verwendet dieselbe config:manage-Regel wie die Anfragenliste", async () => {
    await act(async () => {
      render(<Page params={Promise.resolve({ tenant: "demo", id: "42" })} />);
    });

    expect(await screen.findByText("Detail 42")).toBeInTheDocument();
    expect(useRequirePermission).toHaveBeenCalledWith(
      canReviewEnrollmentChangeRequests,
    );
  });
});
