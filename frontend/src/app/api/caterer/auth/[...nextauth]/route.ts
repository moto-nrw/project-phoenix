import { catererHandlers } from "~/server/auth/caterer";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const GET = withPrivateNoStore(catererHandlers.GET);
export const POST = withPrivateNoStore(catererHandlers.POST);
