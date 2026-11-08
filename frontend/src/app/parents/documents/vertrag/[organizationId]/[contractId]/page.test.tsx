import { act, render, screen } from "@testing-library/react";
import { Suspense } from "react";
import { expect, it, vi } from "vitest";

vi.mock("~/components/parent/care-contract-signing-view", () => ({
  ContractSigningView: () => <div>signing request</div>,
}));
vi.mock("~/components/parent/parent-family-children-card", () => ({
  ParentFamilyChildrenCard: () => <div>family read request</div>,
}));
vi.mock("~/components/parent/parent-page", () => ({
  ParentPage: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ParentPageHeader: () => null,
}));

import Page from "./page";

it("opens the family count without requesting signing permission", async () => {
  const params = Promise.resolve({ organizationId: "4", contractId: "75" });
  const searchParams = Promise.resolve({ family: "1" });
  await act(async () => {
    render(
      <Suspense>
        <Page params={params} searchParams={searchParams} />
      </Suspense>,
    );
  });
  // Anker: the independent family view has loaded.
  expect(await screen.findByText("family read request")).toBeInTheDocument();
  expect(screen.queryByText("signing request")).not.toBeInTheDocument();
});

it("keeps the signing entry point for the normal contract view", async () => {
  const params = Promise.resolve({ organizationId: "4", contractId: "75" });
  const searchParams = Promise.resolve({});
  await act(async () => {
    render(
      <Suspense>
        <Page params={params} searchParams={searchParams} />
      </Suspense>,
    );
  });
  expect(await screen.findByText("signing request")).toBeInTheDocument();
});
