import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import {
  ActiveSupervisionsBreadcrumb,
  EnrollmentBreadcrumb,
  OgsGroupsBreadcrumb,
  PageTitleDisplay,
  ParentChildBreadcrumb,
  RoomBreadcrumb,
  SectionBreadcrumb,
  StaffDetailBreadcrumb,
  StudentDetailBreadcrumb,
  StudentHistoryBreadcrumb,
} from "./breadcrumb-components";

const meta = {
  title: "dashboard/header/BreadcrumbComponents",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const PageTitle: Story = {
  render: () => <PageTitleDisplay title="Meine Gruppe" />,
};

export const PageTitleScrolled: Story = {
  render: () => <PageTitleDisplay title="Meine Gruppe" isScrolled />,
};

export const SectionDatabase: Story = {
  render: () => (
    <SectionBreadcrumb
      sectionLabel="Datenverwaltung"
      sectionHref="/database"
      pageLabel="Personal"
    />
  ),
};

export const SectionDatabaseDeepPage: Story = {
  render: () => (
    <SectionBreadcrumb
      sectionLabel="Datenverwaltung"
      sectionHref="/database"
      pageLabel="Personal"
      pageHref="/database/personal"
      deepLabel="Max Mustermann"
    />
  ),
};

export const SectionPlanungWithoutHub: Story = {
  render: () => (
    <SectionBreadcrumb sectionLabel="Planung" pageLabel="Dienstplan" />
  ),
};

export const SectionEltern: Story = {
  render: () => (
    <SectionBreadcrumb
      sectionLabel="Eltern"
      sectionHref="/eltern"
      pageLabel="Nachrichten"
    />
  ),
};

export const OgsGroups: Story = {
  render: () => <OgsGroupsBreadcrumb />,
};

export const OgsGroupsWithName: Story = {
  render: () => <OgsGroupsBreadcrumb groupName="Gruppe A" />,
};

export const ActiveSupervisions: Story = {
  render: () => <ActiveSupervisionsBreadcrumb />,
};

export const ActiveSupervisionsWithName: Story = {
  render: () => <ActiveSupervisionsBreadcrumb supervisionName="Raum 101" />,
};

export const Enrollment: Story = {
  render: () => <EnrollmentBreadcrumb current="Überblick" />,
};

export const EnrollmentNested: Story = {
  render: () => (
    <EnrollmentBreadcrumb
      current="Phase 1"
      pathname="/admin/enrollments/phases/1"
    />
  ),
};

export const Room: Story = {
  render: () => <RoomBreadcrumb roomName="Turnhalle" />,
};

export const ParentChild: Story = {
  render: () => <ParentChildBreadcrumb childName="Max Mustermann" />,
};

export const StudentHistory: Story = {
  render: () => (
    <StudentHistoryBreadcrumb
      referrer="/students"
      breadcrumbLabel="Schüler"
      pathname="/students/42/history"
      studentName="Max Mustermann"
      historyType="Verlauf"
    />
  ),
};

export const StudentHistoryWithSubSection: Story = {
  render: () => (
    <StudentHistoryBreadcrumb
      referrer="/students"
      breadcrumbLabel="Schüler"
      pathname="/students/42/history"
      studentName="Max Mustermann"
      historyType="Verlauf"
      subSectionName="Anwesenheit"
    />
  ),
};

export const StudentDetail: Story = {
  render: () => (
    <StudentDetailBreadcrumb
      referrer="/students"
      breadcrumbLabel="Schüler"
      studentName="Max Mustermann"
    />
  ),
};

export const StaffDetail: Story = {
  render: () => <StaffDetailBreadcrumb staffName="Erika Musterfrau" />,
};
