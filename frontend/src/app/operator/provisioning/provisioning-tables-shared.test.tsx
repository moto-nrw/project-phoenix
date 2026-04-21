/**
 * Tests for the shared OrgSchoolFilter component.
 *
 * Ported from the "OrgSchoolFilter" describe block in
 * provisioning/page.test.tsx. The filter now lives in this shared module so
 * Konten/Geräte/Personen all use the same primitive.
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { OrgSchoolFilter } from "./provisioning-tables-shared";
import type { Organization, School } from "~/lib/operator/provisioning-helpers";

const org: Organization = {
  id: "1",
  name: "Test Org",
  slug: "test-org",
  active: true,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  deletedAt: null,
} as unknown as Organization;

const school: School = {
  id: "10",
  organizationId: "1",
  name: "Test School",
  slug: "test-school",
  subdomain: "test-school",
  active: true,
  hidden: false,
  deletedAt: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  organization: org,
} as unknown as School;

describe("OrgSchoolFilter", () => {
  it("shows 'Schule auswählen…' when no org is selected", () => {
    render(
      <OrgSchoolFilter
        idPrefix="test"
        organizations={[org]}
        filteredSchools={[school]}
        filterOrgId=""
        selectedSchool={null}
        onOrgChange={() => undefined}
        onSchoolChange={() => undefined}
      />,
    );

    const schoolSelect = screen.getByLabelText("Schule");
    expect(schoolSelect).toHaveTextContent("Schule auswählen…");
  });

  it("shows 'Alle Schulen' when an org is selected", () => {
    render(
      <OrgSchoolFilter
        idPrefix="test"
        organizations={[org]}
        filteredSchools={[school]}
        filterOrgId="1"
        selectedSchool={null}
        onOrgChange={() => undefined}
        onSchoolChange={() => undefined}
      />,
    );

    const schoolSelect = screen.getByLabelText("Schule");
    expect(schoolSelect).toHaveTextContent("Alle Schulen");
  });

  it("shows 'Alle Träger' option in org filter", () => {
    render(
      <OrgSchoolFilter
        idPrefix="test"
        organizations={[org]}
        filteredSchools={[school]}
        filterOrgId=""
        selectedSchool={null}
        onOrgChange={() => undefined}
        onSchoolChange={() => undefined}
      />,
    );

    const orgSelect = screen.getByLabelText("Träger");
    expect(orgSelect).toHaveTextContent("Alle Träger");
  });
});
