import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import {
  PersonalInfoSection,
  HealthInfoSection,
  SupervisorNotesSection,
  AdditionalInfoSection,
  PrivacyConsentSection,
  BusStatusSection,
  PickupStatusSection,
  DepartureSection,
  EnrollmentConsentsSection,
} from "./student-form-fields";

const noopFieldChange = () => {
  // no-op for story
};

const noopStringChange = () => {
  // no-op for story
};

const meta = {
  title: "students/StudentFormFields",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const PersonalInfo: Story = {
  name: "PersonalInfoSection",
  render: () => (
    <PersonalInfoSection formData={{}} onChange={noopFieldChange} errors={{}} />
  ),
};

export const PersonalInfoWithErrors: Story = {
  name: "PersonalInfoSection (with errors)",
  render: () => (
    <PersonalInfoSection
      formData={{ first_name: "Max" }}
      onChange={noopFieldChange}
      errors={{ last_name: "Pflichtfeld" }}
      groups={[{ value: "1", label: "Gruppe A" }]}
    />
  ),
};

export const HealthInfo: Story = {
  name: "HealthInfoSection",
  render: () => <HealthInfoSection value={null} onChange={noopStringChange} />,
};

export const SupervisorNotes: Story = {
  name: "SupervisorNotesSection",
  render: () => (
    <SupervisorNotesSection value={null} onChange={noopStringChange} />
  ),
};

export const AdditionalInfo: Story = {
  name: "AdditionalInfoSection",
  render: () => (
    <AdditionalInfoSection value={null} onChange={noopStringChange} />
  ),
};

export const PrivacyConsent: Story = {
  name: "PrivacyConsentSection",
  render: () => (
    <PrivacyConsentSection
      formData={{}}
      onChange={noopFieldChange}
      errors={{}}
    />
  ),
};

export const BusStatus: Story = {
  name: "BusStatusSection",
  render: () => (
    <BusStatusSection value={false} days={null} onChange={noopFieldChange} />
  ),
};

export const PickupStatus: Story = {
  name: "PickupStatusSection",
  render: () => <PickupStatusSection days={null} onChange={noopFieldChange} />,
};

export const Departure: Story = {
  name: "DepartureSection",
  render: () => <DepartureSection days={null} onChange={noopFieldChange} />,
};

export const EnrollmentConsents: Story = {
  name: "EnrollmentConsentsSection",
  render: () => (
    <EnrollmentConsentsSection
      agbAcceptedAt="2026-01-15"
      dataProcessingAcceptedAt="2026-01-15"
      emailContactAcceptedAt="2026-01-15"
      photoConsentGivenAt={null}
    />
  ),
};
