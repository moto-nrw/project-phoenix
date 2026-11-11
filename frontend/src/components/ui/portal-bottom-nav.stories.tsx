import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import {
  CalendarBlankIcon,
  ChatCircleIcon,
  HouseIcon,
} from "@phosphor-icons/react";

import { PortalBottomNav, PortalBottomNavLink } from "./portal-bottom-nav";

const ITEMS = [
  { key: "home", label: "Start", icon: <HouseIcon size={22} />, active: true },
  {
    key: "calendar",
    label: "Kalender",
    icon: <CalendarBlankIcon size={22} />,
    active: false,
  },
  {
    key: "chat",
    label: "Nachrichten",
    icon: <ChatCircleIcon size={22} />,
    active: false,
  },
];

function Demo() {
  return (
    <div className="min-h-[300px] bg-gray-50">
      <PortalBottomNav listClassName="gap-1" fade={false}>
        {ITEMS.map((item) => (
          <PortalBottomNavLink
            key={item.key}
            href={`#${item.key}`}
            label={item.label}
            icon={item.icon}
            active={item.active}
            paddingClassName="px-3"
            labelClassName="text-sm font-semibold"
          />
        ))}
      </PortalBottomNav>
    </div>
  );
}

const meta = {
  title: "components/ui/PortalBottomNav",
  component: Demo,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof Demo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
