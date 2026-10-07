import { render as renderPlain, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { ToastProvider } from "~/contexts/ToastContext";

const { mockUseSWR } = vi.hoisted(() => ({ mockUseSWR: vi.fn() }));

vi.mock("swr", () => ({ default: mockUseSWR }));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("../provisioning/use-org-school-filter", () => ({
  useOrgSchoolFilter: () => ({
    isAuthenticated: true,
    activeOrganizations: [],
    filterOrgId: "",
    selectedSchool: null,
    filteredSchools: [],
    handleOrgFilterChange: vi.fn(),
    handleSchoolFilterChange: vi.fn(),
    organizationsLoadError: null,
    schoolsLoadError: null,
  }),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: () => null,
}));

vi.mock("../provisioning/provisioning-tables-shared", () => ({
  OrgSchoolFilter: () => null,
}));

vi.mock("../provisioning/provisioning-shared", () => ({
  SimpleEmptyState: ({ title }: { title: string }) => <p>{title}</p>,
}));

vi.mock("~/components/ui/data-table", () => ({
  DataTable: () => <div data-testid="scans-table" />,
}));

import OperatorUnregisteredTagsPage from "./page";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

describe("OperatorUnregisteredTagsPage", () => {
  it("does not show an empty state for stale empty scans after a failed load", async () => {
    mockUseSWR.mockReturnValue({
      data: [],
      error: new ApiError("down", 503),
      isLoading: false,
      mutate: vi.fn(),
    });

    render(<OperatorUnregisteredTagsPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der RFID-Scans"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Keine unbekannten RFID-Scans")).toBeNull();
  });
});
