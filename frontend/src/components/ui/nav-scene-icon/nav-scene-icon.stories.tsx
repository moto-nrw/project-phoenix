import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { UsersIcon } from "@phosphor-icons/react/ssr";

import { NavSceneIcon } from "./nav-scene-icon";
import { childrenConcept } from "~/lib/concepts/children";

const meta = {
  title: "components/ui/NavSceneIcon",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const ActiveAndInactive: Story = {
  render: () => (
    <div className="flex items-center gap-4 text-gray-500">
      <NavSceneIcon icon={UsersIcon} tone="blue" active />
      <NavSceneIcon icon={UsersIcon} tone="blue" />
      <NavSceneIcon
        icon={childrenConcept.icon}
        tone={childrenConcept.tone}
        active
      />
    </div>
  ),
};
