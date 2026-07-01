import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { ProfileDropdownMenu, ProfileTrigger } from "./profile-dropdown";

const meta = {
  title: "dashboard/header/ProfileDropdown",
  parameters: {
    layout: "centered",
  },
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const Trigger: Story = {
  render: () => {
    function TriggerDemo() {
      const [isOpen, setIsOpen] = useState(false);
      return (
        <ProfileTrigger
          displayName="Anna Schmidt"
          displayAvatar={null}
          userRole="Erzieherin"
          isOpen={isOpen}
          onClick={() => setIsOpen((open) => !open)}
        />
      );
    }
    return <TriggerDemo />;
  },
};

export const MenuClosed: Story = {
  render: () => (
    <div className="relative h-40 w-72">
      <ProfileDropdownMenu
        isOpen={false}
        displayName="Anna Schmidt"
        displayAvatar={null}
        userEmail="anna.schmidt@example.com"
        onClose={() => {
          // no-op for the closed-state story
        }}
        onLogout={() => {
          // no-op for the closed-state story
        }}
        profileUrl="/profile"
      />
    </div>
  ),
};

export const MenuOpen: Story = {
  render: () => (
    <div className="relative h-64 w-72">
      <ProfileDropdownMenu
        isOpen={true}
        displayName="Anna Schmidt"
        displayAvatar={null}
        userEmail="anna.schmidt@example.com"
        onClose={() => {
          // no-op for the open-state story
        }}
        onLogout={() => {
          // no-op for the open-state story
        }}
        profileUrl="/profile"
        profileLabel="Mein Profil"
      />
    </div>
  ),
};

export const Composed: Story = {
  render: () => {
    function ComposedDemo() {
      const [isOpen, setIsOpen] = useState(true);
      return (
        <div className="relative inline-block">
          <ProfileTrigger
            displayName="Anna Schmidt"
            displayAvatar={null}
            userRole="Erzieherin"
            isOpen={isOpen}
            onClick={() => setIsOpen((open) => !open)}
          />
          <ProfileDropdownMenu
            isOpen={isOpen}
            displayName="Anna Schmidt"
            displayAvatar={null}
            userEmail="anna.schmidt@example.com"
            onClose={() => setIsOpen(false)}
            onLogout={() => setIsOpen(false)}
            profileUrl="/profile"
          />
        </div>
      );
    }
    return <ComposedDemo />;
  },
};
