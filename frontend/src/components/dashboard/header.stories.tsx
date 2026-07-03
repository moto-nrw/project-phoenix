import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { ProfileProvider } from "~/lib/profile-context";
import { TeacherShellProvider } from "~/lib/shell-auth-context";
import { Header } from "./header";

/**
 * `Header` reads `useShellAuth()` and `useBreadcrumb()`, neither of which is
 * covered by the global Storybook decorator (SessionProvider /
 * NextIntlClientProvider / TenantProvider). Wrap it locally with the real
 * teacher-mode providers: `TeacherShellProvider` derives its user from the
 * global mock session, and needs `ProfileProvider` as an ancestor.
 */
const meta: Meta<typeof Header> = {
  title: "components/dashboard/Header",
  component: Header,
  decorators: [
    (Story) => (
      <BreadcrumbProvider>
        <ProfileProvider>
          <TeacherShellProvider>
            <Story />
          </TeacherShellProvider>
        </ProfileProvider>
      </BreadcrumbProvider>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof Header>;

/**
 * Default header rendered for a logged-in staff member on the dashboard
 * route (`usePathname()` defaults to `/` under the nextjs-vite framework).
 */
export const Default: Story = {};
