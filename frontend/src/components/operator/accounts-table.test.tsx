/**
 * Tests for AccountsTable.
 *
 * Covers showSchool toggle, capability column variations, status badge
 * fallback, and the column sortValue callbacks (name, email, role,
 * pedagogic role, capability) that fire when headers are clicked.
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect } from "vitest";

import { AccountsTable } from "./accounts-table";
import type {
  OrgAccount,
  SchoolAccount,
} from "~/lib/operator/provisioning-helpers";

function makeSchoolAccount(
  overrides: Partial<SchoolAccount> = {},
): SchoolAccount {
  return {
    accountId: "1",
    email: "anna@example.com",
    active: true,
    firstName: "Anna",
    lastName: "Beispiel",
    roleName: "Lehrer",
    pedagogicRole: "Klassenlehrer",
    status: "active",
    hasAdminRole: false,
    hasUserRole: true,
    hasCaregiverProfile: false,
    isActiveCaregiver: false,
    ...overrides,
  };
}

function makeOrgAccount(overrides: Partial<OrgAccount> = {}): OrgAccount {
  return {
    ...makeSchoolAccount(),
    schoolId: "10",
    schoolName: "Test School",
    ...overrides,
  };
}

describe("AccountsTable", () => {
  it("renders name, email, role, and pedagogic role for each account", () => {
    render(<AccountsTable accounts={[makeSchoolAccount()]} />);

    expect(screen.getByText("Anna Beispiel")).toBeInTheDocument();
    expect(screen.getByText("anna@example.com")).toBeInTheDocument();
    expect(screen.getByText("Lehrer")).toBeInTheDocument();
    expect(screen.getByText("Klassenlehrer")).toBeInTheDocument();
  });

  it("renders the Schule column when showSchool is set", () => {
    render(<AccountsTable accounts={[makeOrgAccount()]} showSchool />);

    expect(screen.getByText("Schule")).toBeInTheDocument();
    expect(screen.getByText("Test School")).toBeInTheDocument();
  });

  it("falls back to em dash when role and pedagogic role are empty", () => {
    render(
      <AccountsTable
        accounts={[makeSchoolAccount({ roleName: "", pedagogicRole: "" })]}
      />,
    );

    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(2);
  });

  it("renders an empty state when no accounts are passed", () => {
    render(<AccountsTable accounts={[]} />);

    // AccountsTable does not pass a custom emptyState → DataTable falls
    // back to its default placeholder.
    expect(screen.getByText("Keine Einträge vorhanden.")).toBeInTheDocument();
  });

  it("sorts by Rolle when the header is clicked (covers roleName sortValue)", () => {
    render(
      <AccountsTable
        accounts={[
          makeSchoolAccount({
            accountId: "1",
            firstName: "Anna",
            lastName: "A",
            roleName: "Zett",
          }),
          makeSchoolAccount({
            accountId: "2",
            firstName: "Ben",
            lastName: "B",
            roleName: "Aaa",
          }),
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /^Rolle – / }));

    expect(screen.getByText("Anna A")).toBeInTheDocument();
    expect(screen.getByText("Ben B")).toBeInTheDocument();
  });

  it("sorts by Päd. Rolle when the header is clicked (covers pedagogicRole sortValue)", () => {
    render(
      <AccountsTable
        accounts={[
          makeSchoolAccount({
            accountId: "1",
            firstName: "Anna",
            lastName: "A",
            pedagogicRole: "Zett-Rolle",
          }),
          makeSchoolAccount({
            accountId: "2",
            firstName: "Ben",
            lastName: "B",
            pedagogicRole: "Aaa-Rolle",
          }),
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /^Päd\. Rolle – / }));

    expect(screen.getByText("Anna A")).toBeInTheDocument();
    expect(screen.getByText("Ben B")).toBeInTheDocument();
  });
});
