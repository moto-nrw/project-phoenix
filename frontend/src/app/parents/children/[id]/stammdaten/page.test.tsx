import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import ParentChildMasterDataPage from "./page";

vi.mock("react", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react")>();
  return {
    ...actual,
    use: () => ({ id: "42" }),
  };
});

vi.mock("~/components/parent/child-master-data", () => ({
  ChildMasterDataView: ({ studentId }: { studentId: string }) => (
    <div>child-master-data:{studentId}</div>
  ),
}));

describe("ParentChildMasterDataPage", () => {
  it("passes the route id to the child master-data view", () => {
    render(
      <ParentChildMasterDataPage params={Promise.resolve({ id: "42" })} />,
    );

    expect(screen.getByText("child-master-data:42")).toBeInTheDocument();
  });
});
