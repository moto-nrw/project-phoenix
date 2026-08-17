import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DatabaseSectionSkeleton } from "./section-skeleton";

const mockUseSelectedLayoutSegments = vi.fn();

vi.mock("next/navigation", () => ({
  useSelectedLayoutSegments: () => mockUseSelectedLayoutSegments(),
}));

vi.mock("~/components/database/master-detail-skeleton", () => ({
  MasterDetailSkeleton: () => <div>master-detail</div>,
}));

vi.mock("./page-skeleton", () => ({
  DatabaseIndexSkeleton: () => <div>database-index</div>,
}));

describe("DatabaseSectionSkeleton", () => {
  beforeEach(() => {
    mockUseSelectedLayoutSegments.mockReset();
  });

  it("matches the database index", () => {
    mockUseSelectedLayoutSegments.mockReturnValue([]);

    render(<DatabaseSectionSkeleton />);

    expect(screen.getByText("database-index")).toBeInTheDocument();
  });

  it("matches direct master-detail routes", () => {
    mockUseSelectedLayoutSegments.mockReturnValue(["students"]);

    render(<DatabaseSectionSkeleton />);

    expect(screen.getByText("master-detail")).toBeInTheDocument();
  });

  it("uses neutral progress for other database routes", () => {
    mockUseSelectedLayoutSegments.mockReturnValue(["students", "import"]);

    render(<DatabaseSectionSkeleton />);

    const loading = screen.getByRole("status", {
      name: "Datenverwaltung wird geladen…",
    });
    expect(loading).not.toHaveClass("fixed");
  });
});
