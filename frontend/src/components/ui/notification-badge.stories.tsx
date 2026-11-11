import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { NotificationBadge } from "./notification-badge";

const meta = {
  title: "components/ui/NotificationBadge",
} satisfies Meta;

export default meta;
type Story = StoryObj;

// "parents" is the same blue as "blue", so the story shows it once.
const TONES = ["staff", "feedback", "danger", "blue"] as const;

export const AllTones: Story = {
  render: () => (
    <div className="flex flex-col gap-3">
      {(["sm", "md"] as const).map((size) => (
        <div key={size} className="flex items-center gap-3">
          {TONES.map((tone) => (
            <NotificationBadge
              key={tone}
              count={7}
              tone={tone}
              size={size}
              ariaLabel={`7 neue Einträge (${tone})`}
            />
          ))}
        </div>
      ))}
    </div>
  ),
};
