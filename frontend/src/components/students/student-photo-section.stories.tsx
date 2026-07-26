import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Student } from "~/lib/api";

import { StudentPhotoSection } from "./student-photo-section";

const baseStudent: Student = {
  id: "1",
  name: "Max Mustermann",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "3a",
  current_location: "group_room",
};

const meta = {
  title: "students/StudentPhotoSection",
  component: StudentPhotoSection,
  parameters: {
    layout: "centered",
  },
} satisfies Meta<typeof StudentPhotoSection>;

export default meta;

type Story = StoryObj<typeof meta>;

export const NoConsentNoPhoto: Story = {
  args: {
    student: baseStudent,
    consentGiven: false,
    onConsentChange: () => {
      // no-op for story
    },
    pendingPhotoBlob: null,
    pendingPhotoRemoved: false,
    onPickPhoto: () => {
      // no-op for story
    },
    onMarkRemoved: () => {
      // no-op for story
    },
    onCancelRemove: () => {
      // no-op for story
    },
  },
};

export const ConsentGivenNoPhoto: Story = {
  args: {
    student: {
      ...baseStudent,
      photo_consent_given_at: "2026-05-12T10:30:00Z",
    },
    consentGiven: true,
    onConsentChange: () => {
      // no-op for story
    },
    pendingPhotoBlob: null,
    pendingPhotoRemoved: false,
    onPickPhoto: () => {
      // no-op for story
    },
    onMarkRemoved: () => {
      // no-op for story
    },
    onCancelRemove: () => {
      // no-op for story
    },
  },
};

export const ConsentGivenWithServerPhoto: Story = {
  args: {
    student: {
      ...baseStudent,
      photo_url: "https://placehold.co/128x128",
      photo_consent_given_at: "2026-05-12T10:30:00Z",
    },
    consentGiven: true,
    onConsentChange: () => {
      // no-op for story
    },
    pendingPhotoBlob: null,
    pendingPhotoRemoved: false,
    onPickPhoto: () => {
      // no-op for story
    },
    onMarkRemoved: () => {
      // no-op for story
    },
    onCancelRemove: () => {
      // no-op for story
    },
  },
};

export const PendingRemoval: Story = {
  args: {
    student: {
      ...baseStudent,
      photo_url: "https://placehold.co/128x128",
      photo_consent_given_at: "2026-05-12T10:30:00Z",
    },
    consentGiven: true,
    onConsentChange: () => {
      // no-op for story
    },
    pendingPhotoBlob: null,
    pendingPhotoRemoved: true,
    onPickPhoto: () => {
      // no-op for story
    },
    onMarkRemoved: () => {
      // no-op for story
    },
    onCancelRemove: () => {
      // no-op for story
    },
  },
};
