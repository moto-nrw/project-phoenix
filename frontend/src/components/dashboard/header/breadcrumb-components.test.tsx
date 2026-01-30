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
    render(<OgsGroupsBreadcrumb groupName="Test Group" />);

    expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
    expect(screen.getByText("Test Group")).toBeInTheDocument();
  });
});

describe("ActiveSupervisionsBreadcrumb", () => {
  it("renders simple title when no supervision name", () => {
    render(<ActiveSupervisionsBreadcrumb />);

    expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
  });

  it("renders breadcrumb with supervision name", () => {
    render(<ActiveSupervisionsBreadcrumb supervisionName="Room 101" />);

    expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
    expect(screen.getByText("Room 101")).toBeInTheDocument();
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
    render(<ActivityBreadcrumb activityName="Fußball" />);

    expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
    expect(screen.getByText("Fußball")).toBeInTheDocument();
  });

  it("links to activities page", () => {
    render(<ActivityBreadcrumb activityName="Fußball" />);

    const activitiesLink = screen.getByRole("link", { name: "Aktivitäten" });
    expect(activitiesLink).toHaveAttribute("href", "/activities");
  });
});

describe("RoomBreadcrumb", () => {
  it("renders room breadcrumb", () => {
    render(<RoomBreadcrumb roomName="Room 101" />);

    expect(screen.getByText("Räume")).toBeInTheDocument();
    expect(screen.getByText("Room 101")).toBeInTheDocument();
  });

  it("links to rooms page", () => {
    render(<RoomBreadcrumb roomName="Room 101" />);

    const roomsLink = screen.getByRole("link", { name: "Räume" });
    expect(roomsLink).toHaveAttribute("href", "/rooms");
  });
});

describe("StudentHistoryBreadcrumb", () => {
  it("renders three-level breadcrumb", () => {
    render(
      <StudentHistoryBreadcrumb
        referrer="/database/students"
        breadcrumbLabel="Schüler"
        pathname="/database/students/123/history"
        studentName="Max Mustermann"
        historyType="Verlauf"
      />,
    );

    expect(screen.getByText("Schüler")).toBeInTheDocument();
    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    expect(screen.getByText("Verlauf")).toBeInTheDocument();
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
  it("renders two-level breadcrumb", () => {
    render(
      <StudentDetailBreadcrumb
        referrer="/database/students"
        breadcrumbLabel="Schüler"
        studentName="Max Mustermann"
      />,
    );

    expect(screen.getByText("Schüler")).toBeInTheDocument();
    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
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
