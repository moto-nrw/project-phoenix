// Client-side API of the caterer's two-factor sign-in (#4497) on the account
// page. It stays out of caterer-api so the account page does not load the
// order and delivery-list code. Every call goes through a Next.js route under
// /api/caterer/account/mfa, which forwards it with the caterer session.

import { emailCodeFactorApi } from "./email-code-factor-api";

const routes = emailCodeFactorApi("/api/caterer/account/mfa");

export const fetchCatererMFAStatus = () => routes.status();
export const startCatererMFA = () => routes.start();
export const confirmCatererMFA = (challengeToken: string, code: string) =>
  routes.confirm(challengeToken, code);
export const disableCatererMFA = (currentPassword: string) =>
  routes.disable(currentPassword);
