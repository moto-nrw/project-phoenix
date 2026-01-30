import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
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
    href,
    children,
    onClick,
    className,
  }: {
    href: string;
    children: React.ReactNode;
    onClick?: () => void;
    className?: string;
  }) => (
    <a href={href} onClick={onClick} className={className}>
      {children}
    </a>
  ),
}));

describe("PageTitleDisplay", () => {
  it("renders title text", () => {
    render(<PageTitleDisplay title="Dashboard" />);
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
  });

  it("uses text-base when not scrolled", () => {
    render(<PageTitleDisplay title="Dashboard" isScrolled={false} />);
    const el = screen.getByText("Dashboard");
    expect(el.className).toContain("text-base");
  });

  it("uses text-sm when scrolled", () => {
    render(<PageTitleDisplay title="Dashboard" isScrolled={true} />);
    const el = screen.getByText("Dashboard");
    expect(el.className).toContain("text-sm");
  });
});

describe("DatabaseBreadcrumb", () => {
  it("renders simple breadcrumb when not deep page", () => {
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

  it("renders deep breadcrumb with sub-page", () => {
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
});

describe("OgsGroupsBreadcrumb", () => {
  it("shows breadcrumb trail when groupName is provided", () => {
    render(<OgsGroupsBreadcrumb groupName="Eulen" />);
    expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
    expect(screen.getByText("Eulen")).toBeInTheDocument();
  });

  it("shows simple title when no groupName", () => {
    render(<OgsGroupsBreadcrumb />);
    expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
  });
});

describe("ActiveSupervisionsBreadcrumb", () => {
  it("shows breadcrumb trail when supervisionName is provided", () => {
    render(<ActiveSupervisionsBreadcrumb supervisionName="Raum 1.2" />);
    expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
    expect(screen.getByText("Raum 1.2")).toBeInTheDocument();
  });

  it("shows simple title when no supervisionName", () => {
    render(<ActiveSupervisionsBreadcrumb />);
    expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
  });
});

describe("InvitationsBreadcrumb", () => {
  it("renders three-level breadcrumb", () => {
    render(<InvitationsBreadcrumb />);
    expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    expect(screen.getByText("Betreuer")).toBeInTheDocument();
    expect(screen.getByText("Einladungen")).toBeInTheDocument();
  });
});

describe("ActivityBreadcrumb", () => {
  it("renders activity breadcrumb", () => {
    render(<ActivityBreadcrumb activityName="Fußball AG" />);
    expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
    expect(screen.getByText("Fußball AG")).toBeInTheDocument();
  });
});

describe("RoomBreadcrumb", () => {
  it("renders room breadcrumb", () => {
    render(<RoomBreadcrumb roomName="Sporthalle" />);
    expect(screen.getByText("Räume")).toBeInTheDocument();
    expect(screen.getByText("Sporthalle")).toBeInTheDocument();
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

  it("applies scrolled text size", () => {
    render(
      <StudentDetailBreadcrumb
        referrer="/students/search"
        breadcrumbLabel="Suche"
        studentName="Emma"
        isScrolled={true}
      />,
    );
    const nav = document.querySelector("nav");
    expect(nav?.className).toContain("text-sm");
  });
});
