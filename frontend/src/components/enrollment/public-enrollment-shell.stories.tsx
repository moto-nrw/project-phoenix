import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ShieldCheck } from "lucide-react";
import type { TenantInfo } from "~/lib/tenant-api";
import {
  PublicEnrollmentBackLink,
  PublicEnrollmentBrand,
  PublicEnrollmentLocaleSwitcher,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
  PublicInfoCard,
} from "./public-enrollment-shell";

const fakeTenant: TenantInfo = {
  tenantId: 1,
  slug: "musterschule",
  name: "OGS Musterschule",
  subdomain: "musterschule",
  organizationId: 1,
  organizationName: "Muster Träger",
  settings: {},
  presenceMode: "detailed",
  studentPhotosEnabled: false,
  nfcEnabled: true,
  messagingEnabled: false,
  displayEnabled: false,
  gradeLevelMax: 4,
};

const meta = {
  title: "components/enrollment/PublicEnrollmentShell",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const Brand: Story = {
  render: () => <PublicEnrollmentBrand tenant={fakeTenant} />,
};

export const BrandFallback: Story = {
  render: () => <PublicEnrollmentBrand tenant={null} />,
};

export const StepsPhase: Story = {
  render: () => <PublicEnrollmentSteps current="phase" />,
};

export const StepsForm: Story = {
  render: () => <PublicEnrollmentSteps current="form" />,
};

export const StepsDone: Story = {
  render: () => <PublicEnrollmentSteps current="done" />,
};

export const LocaleSwitcher: Story = {
  render: () => <PublicEnrollmentLocaleSwitcher />,
};

export const BackLink: Story = {
  render: () => (
    <PublicEnrollmentBackLink href="#">Zurück</PublicEnrollmentBackLink>
  ),
};

export const InfoCard: Story = {
  render: () => (
    <PublicInfoCard icon={<ShieldCheck className="h-5 w-5" />} title="DSGVO">
      Ihre Daten werden gemäß den geltenden Datenschutzbestimmungen verarbeitet.
    </PublicInfoCard>
  ),
};

export const Composition: Story = {
  render: () => (
    <PublicEnrollmentPageShell withInlineSwitcher>
      <div className="mb-6 flex items-center justify-between gap-4">
        <PublicEnrollmentBrand tenant={fakeTenant} />
        <div className="flex items-center gap-3">
          <PublicEnrollmentSteps current="form" />
          <PublicEnrollmentLocaleSwitcher />
        </div>
      </div>
      <PublicEnrollmentBackLink href="#">Zurück</PublicEnrollmentBackLink>
      <div className="mt-4">
        <PublicInfoCard
          icon={<ShieldCheck className="h-5 w-5" />}
          title="DSGVO"
        >
          Ihre Daten werden gemäß den geltenden Datenschutzbestimmungen
          verarbeitet.
        </PublicInfoCard>
      </div>
    </PublicEnrollmentPageShell>
  ),
};
