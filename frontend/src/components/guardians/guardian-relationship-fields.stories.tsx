import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import {
  GuardianRoleSelect,
  RelationshipPermissionsFields,
  RelationshipTypeSelect,
} from "./guardian-relationship-fields";
import type { RelationshipFlag } from "./guardian-relationship-fields";
import type { GuardianRole as GuardianRoleType } from "@/lib/guardian-helpers";

const meta = {
  title: "components/guardians/GuardianRelationshipFields",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const RelationshipType: Story = {
  render: function Render() {
    const [value, setValue] = useState("parent");
    return (
      <RelationshipTypeSelect
        id="relationship-type"
        value={value}
        onChange={setValue}
      />
    );
  },
};

export const RelationshipTypeDisabled: Story = {
  render: function Render() {
    const [value, setValue] = useState("parent");
    return (
      <RelationshipTypeSelect
        id="relationship-type-disabled"
        value={value}
        onChange={setValue}
        disabled
      />
    );
  },
};

export const GuardianRole: Story = {
  render: function Render() {
    const [value, setValue] = useState<GuardianRoleType>("legal_guardian");
    return (
      <GuardianRoleSelect
        id="guardian-role"
        value={value}
        onChange={setValue}
      />
    );
  },
};

export const PermissionsFields: Story = {
  render: function Render() {
    const [flags, setFlags] = useState<Record<RelationshipFlag, boolean>>({
      isPrimary: false,
      canPickup: false,
      isEmergencyContact: false,
    });
    return (
      <RelationshipPermissionsFields
        isPrimary={flags.isPrimary}
        canPickup={flags.canPickup}
        isEmergencyContact={flags.isEmergencyContact}
        onChange={(field, value) =>
          setFlags((prev) => ({ ...prev, [field]: value }))
        }
      />
    );
  },
};

export const Composition: Story = {
  render: function Render() {
    const [relationshipType, setRelationshipType] = useState("parent");
    const [role, setRole] = useState<GuardianRoleType>("legal_guardian");
    const [flags, setFlags] = useState<Record<RelationshipFlag, boolean>>({
      isPrimary: true,
      canPickup: false,
      isEmergencyContact: false,
    });
    return (
      <div className="max-w-md space-y-4">
        <RelationshipTypeSelect
          id="composition-relationship-type"
          value={relationshipType}
          onChange={setRelationshipType}
        />
        <GuardianRoleSelect
          id="composition-guardian-role"
          value={role}
          onChange={setRole}
        />
        <RelationshipPermissionsFields
          isPrimary={flags.isPrimary}
          canPickup={flags.canPickup}
          isEmergencyContact={flags.isEmergencyContact}
          onChange={(field, value) =>
            setFlags((prev) => ({ ...prev, [field]: value }))
          }
        />
      </div>
    );
  },
};
