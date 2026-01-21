/**
 * Email utilities for BetterAuth organization lifecycle notifications.
 * Sends emails via the Go backend's internal API endpoint.
 */

const INTERNAL_API_URL =
  process.env.INTERNAL_API_URL ?? "http://server:8080";
const BASE_DOMAIN = process.env.BASE_DOMAIN ?? "moto-app.de";

interface SendEmailParams {
  to: string;
  template: string;
  subject?: string;
  data: Record<string, string | undefined>;
}

/**
 * Send an email via the internal API endpoint.
 * This is a fire-and-forget operation - errors are logged but don't throw.
 */
async function sendEmail(params: SendEmailParams): Promise<void> {
  const url = `${INTERNAL_API_URL}/api/internal/email`;

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        to: params.to,
        template: params.template,
        subject: params.subject,
        data: params.data,
      }),
    });

    if (!response.ok) {
      const errorText = await response.text();
      console.error(
        `Failed to send email (${params.template}):`,
        response.status,
        errorText,
      );
    } else {
      console.log(`Email sent successfully: ${params.template} to ${params.to}`);
    }
  } catch (error) {
    console.error(`Failed to send email (${params.template}):`, error);
  }
}

interface OrgPendingEmailParams {
  to: string;
  firstName?: string;
  orgName: string;
  subdomain: string;
}

/**
 * Send organization pending notification email.
 * Sent to the organization creator when a new organization is created.
 */
export async function sendOrgPendingEmail(
  params: OrgPendingEmailParams,
): Promise<void> {
  const orgURL = `https://${params.subdomain}.${BASE_DOMAIN}`;

  await sendEmail({
    to: params.to,
    template: "org-pending",
    data: {
      FirstName: params.firstName,
      OrgName: params.orgName,
      Subdomain: params.subdomain,
      OrgURL: orgURL,
    },
  });
}

interface OrgApprovedEmailParams {
  to: string;
  firstName?: string;
  orgName: string;
  subdomain: string;
}

/**
 * Send organization approved notification email.
 * Sent to the organization owner when their organization is approved.
 */
export async function sendOrgApprovedEmail(
  params: OrgApprovedEmailParams,
): Promise<void> {
  const orgURL = `https://${params.subdomain}.${BASE_DOMAIN}`;

  await sendEmail({
    to: params.to,
    template: "org-approved",
    data: {
      FirstName: params.firstName,
      OrgName: params.orgName,
      Subdomain: params.subdomain,
      OrgURL: orgURL,
    },
  });
}

interface OrgRejectedEmailParams {
  to: string;
  firstName?: string;
  orgName: string;
  reason?: string;
}

/**
 * Send organization rejected notification email.
 * Sent to the organization owner when their organization is rejected.
 */
export async function sendOrgRejectedEmail(
  params: OrgRejectedEmailParams,
): Promise<void> {
  await sendEmail({
    to: params.to,
    template: "org-rejected",
    data: {
      FirstName: params.firstName,
      OrgName: params.orgName,
      Reason: params.reason,
    },
  });
}
