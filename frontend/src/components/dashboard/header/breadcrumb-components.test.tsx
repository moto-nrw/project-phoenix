/**
 * Tests for Breadcrumb Components
 * Tests rendering and navigation breadcrumb patterns
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import {
  PageTitleDisplay,
  DatabaseBreadcrumb,
  OgsGroupsBreadcrumb,
  ActiveSupervisionsBreadcrumb,
  InvitationsBreadcrumb,
  ActivityBreadcrumb,
  RoomBreadcrumb,
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

describe("DatabaseBreadcrumb", () => {
  it("renders database breadcrumb with page title", () => {
    render(
      <DatabaseBreadcrumb
        pathname="/database/students"
        pageTitle="Schüler"
        subPageLabel=""
        isDeepPage={false}
      />,
    );

    expect(screen.getByText("Datenbank")).toBeInTheDocument();
    expect(screen.getByText("Schüler")).toBeInTheDocument();
  });

  it("renders deep page with three levels", () => {
    render(
      <DatabaseBreadcrumb
        pathname="/database/students/123"
        pageTitle="Schüler"
        subPageLabel="Details"
        isDeepPage={true}
      />,
    );

    expect(screen.getByText("Datenbank")).toBeInTheDocument();
    expect(screen.getByText("Schüler")).toBeInTheDocument();
    expect(screen.getByText("Details")).toBeInTheDocument();
  });

  it("links correctly", () => {
    render(
      <DatabaseBreadcrumb
        pathname="/database/students"
        pageTitle="Schüler"
        subPageLabel=""
        isDeepPage={false}
      />,
    );

    const databaseLink = screen.getByRole("link", { name: "Datenbank" });
    expect(databaseLink).toHaveAttribute("href", "/database");
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

describe("InvitationsBreadcrumb", () => {
  it("renders three-level breadcrumb", () => {
    render(<InvitationsBreadcrumb />);

    expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    expect(screen.getByText("Betreuer")).toBeInTheDocument();
    expect(screen.getByText("Einladungen")).toBeInTheDocument();
  });

  it("links correctly", () => {
    render(<InvitationsBreadcrumb />);

    const databaseLink = screen.getByRole("link", { name: "Datenverwaltung" });
    expect(databaseLink).toHaveAttribute("href", "/database");

    const teachersLink = screen.getByRole("link", { name: "Betreuer" });
    expect(teachersLink).toHaveAttribute("href", "/database/teachers");
  });
});

describe("ActivityBreadcrumb", () => {
  it("renders activity breadcrumb", () => {
    render(<ActivityBreadcrumb activityName="Fußball AG" />);

    expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
    expect(screen.getByText("Fußball AG")).toBeInTheDocument();
  });

  it("links to activities page", () => {
    render(<ActivityBreadcrumb activityName="Fußball" />);

    const activitiesLink = screen.getByRole("link", { name: "Aktivitäten" });
    expect(activitiesLink).toHaveAttribute("href", "/activities");
  });
});

describe("RoomBreadcrumb", () => {
  it("renders room breadcrumb", () => {
    render(<RoomBreadcrumb roomName="Sporthalle" />);

    expect(screen.getByText("Räume")).toBeInTheDocument();
    expect(screen.getByText("Sporthalle")).toBeInTheDocument();
  });

  it("links to rooms page", () => {
    render(<RoomBreadcrumb roomName="Sporthalle" />);

    const roomsLink = screen.getByRole("link", { name: "Räume" });
    expect(roomsLink).toHaveAttribute("href", "/rooms");
  });
});

describe("StudentHistoryBreadcrumb", () => {
  it("renders history breadcrumb without sub-section", () => {
    render(
      <StudentHistoryBreadcrumb
        referrer="/students/search"
        breadcrumbLabel="Suche"
        pathname="/students/123/feedback_history"
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
        pathname="/students/123/room_history"
        studentName="Max"
        historyType="Raumverlauf"
        subSectionName="Eulen"
      />,
    );
    expect(screen.getByText("Eulen")).toBeInTheDocument();
    expect(screen.getByText("Max")).toBeInTheDocument();
    expect(screen.getByText("Raumverlauf")).toBeInTheDocument();
  });

  it("links correctly", () => {
    render(
      <StudentHistoryBreadcrumb
        referrer="/database/students"
        breadcrumbLabel="Schüler"
        pathname="/database/students/123/history"
        studentName="Max Mustermann"
        historyType="Verlauf"
      />,
    );

    const referrerLink = screen.getByRole("link", { name: "Schüler" });
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
        breadcrumbLabel="Schüler"
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
        breadcrumbLabel="Schüler"
        studentName="Max Mustermann"
      />,
    );

    const link = screen.getByRole("link", { name: "Schüler" });
    fireEvent.click(link);
    // Just verify it doesn't crash
    expect(container).toBeInTheDocument();
  });
});
