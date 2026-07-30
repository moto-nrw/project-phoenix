/**
 * Tests for Breadcrumb Components
 * Tests rendering and navigation breadcrumb patterns
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import {
  PageTitleDisplay,
  SectionBreadcrumb,
  OgsGroupsBreadcrumb,
  ActiveSupervisionsBreadcrumb,
  EnrollmentBreadcrumb,
  StudentHistoryBreadcrumb,
  StudentDetailBreadcrumb,
} from "./breadcrumb-components";

// Mock next/link
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    onClick,
    className,
  }: {
    children: React.ReactNode;
    href: string;
    onClick?: () => void;
    className?: string;
  }) => (
    <a href={href} onClick={onClick} className={className}>
      {children}
    </a>
  ),
}));

describe("PageTitleDisplay", () => {
  it("renders page title", () => {
    render(<PageTitleDisplay title="Test Page" />);

    expect(screen.getByText("Test Page")).toBeInTheDocument();
  });

  it("uses text-base when not scrolled", () => {
    render(<PageTitleDisplay title="Dashboard" isScrolled={false} />);
    const el = screen.getByText("Dashboard");
    expect(el.className).toContain("text-base");
  });

  it("applies smaller text when scrolled", () => {
    render(<PageTitleDisplay title="Test Page" isScrolled={true} />);

    const title = screen.getByText("Test Page");
    expect(title).toHaveClass("text-sm");
  });

  it("is hidden on mobile", () => {
    render(<PageTitleDisplay title="Test Page" />);

    const title = screen.getByText("Test Page");
    expect(title).toHaveClass("hidden");
  });
});

describe("SectionBreadcrumb", () => {
  it("renders section breadcrumb with page title", () => {
    render(
      <SectionBreadcrumb
        sectionLabel="Datenverwaltung"
        sectionHref="/database"
        pageLabel="Kinder"
      />,
    );

    expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    expect(screen.getByText("Kinder")).toBeInTheDocument();
  });

  it("renders deep page with three levels", () => {
    render(
      <SectionBreadcrumb
        sectionLabel="Datenverwaltung"
        sectionHref="/database"
        pageLabel="Kinder"
        pageHref="/database/students"
        deepLabel="Details"
      />,
    );

    expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    expect(screen.getByText("Kinder")).toBeInTheDocument();
    expect(screen.getByText("Details")).toBeInTheDocument();
  });

  it("links correctly", () => {
    render(
      <SectionBreadcrumb
        sectionLabel="Datenverwaltung"
        sectionHref="/database"
        pageLabel="Kinder"
      />,
    );

    const sectionLink = screen.getByRole("link", { name: "Datenverwaltung" });
    expect(sectionLink).toHaveAttribute("href", "/database");
  });

  it("renders the section as plain text when no sectionHref is given", () => {
    render(<SectionBreadcrumb sectionLabel="Planung" pageLabel="Dienstplan" />);

    expect(screen.getByText("Planung")).toBeInTheDocument();
    expect(screen.getByText("Dienstplan")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Planung" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("renders an Eltern section breadcrumb", () => {
    render(
      <SectionBreadcrumb
        sectionLabel="Eltern"
        sectionHref="/eltern"
        pageLabel="Nachrichten"
      />,
    );

    expect(screen.getByText("Eltern")).toBeInTheDocument();
    expect(screen.getByText("Nachrichten")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Eltern" })).toHaveAttribute(
      "href",
      "/eltern",
    );
  });
});

describe("OgsGroupsBreadcrumb", () => {
  it("renders simple title when no group name", () => {
    render(<OgsGroupsBreadcrumb />);

    expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
  });

  it("renders breadcrumb with group name", () => {
    render(<OgsGroupsBreadcrumb groupName="Eulen" />);

    expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
    expect(screen.getByText("Eulen")).toBeInTheDocument();
  });
});

describe("ActiveSupervisionsBreadcrumb", () => {
  it("renders simple title when no supervision name", () => {
    render(<ActiveSupervisionsBreadcrumb />);

    expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
  });

  it("renders breadcrumb with supervision name", () => {
    render(<ActiveSupervisionsBreadcrumb supervisionName="Raum 1.2" />);

    expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
    expect(screen.getByText("Raum 1.2")).toBeInTheDocument();
  });
});

describe("EnrollmentBreadcrumb", () => {
  it("renders enrollment parent and current page", () => {
    render(<EnrollmentBreadcrumb current="Anmeldeformulare" />);

    expect(screen.getByText("Anmeldungen")).toBeInTheDocument();
    expect(screen.getByText("Anmeldeformulare")).toBeInTheDocument();
  });

  it("links back to the enrollment process page", () => {
    render(<EnrollmentBreadcrumb current="Betreuungsangebote" />);

    const link = screen.getByRole("link", { name: "Anmeldungen" });
    expect(link).toHaveAttribute("href", "/admin/enrollments");
  });

  it("renders nested enrollment detail breadcrumbs", () => {
    render(
      <EnrollmentBreadcrumb
        current="Schuljahr 2026/2027"
        pathname="/admin/enrollments/phases/phase-1"
      />,
    );

    expect(screen.getByText("Anmeldungen")).toBeInTheDocument();
    expect(screen.getByText("Überblick")).toBeInTheDocument();
    expect(screen.getByText("Schuljahr 2026/2027")).toBeInTheDocument();
  });
});

describe("StudentHistoryBreadcrumb", () => {
  it("renders history breadcrumb without sub-section", () => {
    render(
      <StudentHistoryBreadcrumb
        referrer="/students/search"
        breadcrumbLabel="Suche"
        pathname="/students/123/feedback-history"
        studentName="Emma Müller"
        historyType="Feedbackhistorie"
      />,
    );
    expect(screen.getByText("Suche")).toBeInTheDocument();
    expect(screen.getByText("Emma Müller")).toBeInTheDocument();
    expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
  });

  it("renders history breadcrumb with sub-section name", () => {
    render(
      <StudentHistoryBreadcrumb
        referrer="/ogs-groups"
        breadcrumbLabel="Meine Gruppe"
        pathname="/students/123/room-history"
        studentName="Max"
        historyType="Anwesenheitsprotokoll"
        subSectionName="Eulen"
      />,
    );
    expect(screen.getByText("Eulen")).toBeInTheDocument();
    expect(screen.getByText("Max")).toBeInTheDocument();
    expect(screen.getByText("Anwesenheitsprotokoll")).toBeInTheDocument();
  });

  it("links correctly", () => {
    render(
      <StudentHistoryBreadcrumb
        referrer="/database/students"
        breadcrumbLabel="Kinder"
        pathname="/database/students/123/history"
        studentName="Max Mustermann"
        historyType="Verlauf"
      />,
    );

    const referrerLink = screen.getByRole("link", { name: "Kinder" });
    expect(referrerLink).toHaveAttribute("href", "/database/students");
  });
});

describe("StudentDetailBreadcrumb", () => {
  it("renders detail breadcrumb without sub-section", () => {
    render(
      <StudentDetailBreadcrumb
        referrer="/students/search"
        breadcrumbLabel="Suche"
        studentName="Emma Müller"
      />,
    );
    expect(screen.getByText("Suche")).toBeInTheDocument();
    expect(screen.getByText("Emma Müller")).toBeInTheDocument();
  });

  it("renders detail breadcrumb with sub-section name", () => {
    render(
      <StudentDetailBreadcrumb
        referrer="/ogs-groups"
        breadcrumbLabel="Meine Gruppe"
        studentName="Max"
        subSectionName="Eulen"
      />,
    );
    expect(screen.getByText("Eulen")).toBeInTheDocument();
    expect(screen.getByText("Max")).toBeInTheDocument();
  });

  it("applies smaller text when scrolled", () => {
    const { container } = render(
      <StudentDetailBreadcrumb
        referrer="/database/students"
        breadcrumbLabel="Kinder"
        studentName="Max Mustermann"
        isScrolled={true}
      />,
    );

    const nav = container.querySelector("nav");
    expect(nav).toHaveClass("text-sm");
  });

  it("handles onClick for links", () => {
    const { container } = render(
      <StudentDetailBreadcrumb
        referrer="/database/students"
        breadcrumbLabel="Kinder"
        studentName="Max Mustermann"
      />,
    );

    const link = screen.getByRole("link", { name: "Kinder" });
    fireEvent.click(link);
    // Just verify it doesn't crash
    expect(container).toBeInTheDocument();
  });
});
