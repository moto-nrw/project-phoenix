import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Bell } from "lucide-react";
import { DesktopTabsActionArea, MobileTabsActionArea } from "./TabsActionArea";
import { Button } from "~/components/ui/button";

const meta: Meta<typeof DesktopTabsActionArea> = {
  title: "components/ui/page-header/TabsActionArea",
  component: DesktopTabsActionArea,
};

export default meta;

type Story = StoryObj<typeof DesktopTabsActionArea>;

export const DesktopWithActionButton: Story = {
  render: () => (
    <DesktopTabsActionArea
      hasTitle={false}
      actionButton={
        <Button type="button" size="compact" variant="ghost">
          Aktion
        </Button>
      }
    />
  ),
};

export const DesktopWithStatusIndicator: Story = {
  render: () => (
    <DesktopTabsActionArea
      hasTitle={false}
      statusIndicator={{ color: "green", tooltip: "Online" }}
    />
  ),
};

export const DesktopWithBadge: Story = {
  render: () => (
    <DesktopTabsActionArea
      hasTitle={false}
      badge={{ count: 3, label: "Neu", icon: <Bell className="h-4 w-4" /> }}
    />
  ),
};

export const DesktopHiddenWhenHasTitleWithoutAction: Story = {
  render: () => (
    <DesktopTabsActionArea hasTitle={true} badge={{ count: 3, label: "Neu" }} />
  ),
};

export const MobileWithActionButton: Story = {
  render: () => (
    <MobileTabsActionArea
      hasTitle={false}
      actionButton={
        <Button type="button" size="icon" variant="ghost">
          <Bell className="h-4 w-4" />
        </Button>
      }
    />
  ),
};

export const MobileWithBadgeCompact: Story = {
  render: () => <MobileTabsActionArea hasTitle={false} badge={{ count: 12 }} />,
};

export const MobileHiddenWhenHasTitle: Story = {
  render: () => <MobileTabsActionArea hasTitle={true} badge={{ count: 12 }} />,
};
