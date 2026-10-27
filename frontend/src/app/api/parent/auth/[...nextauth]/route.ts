import { parentHandlers } from "~/server/auth/parent";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const GET = withPrivateNoStore(parentHandlers.GET);
export const POST = withPrivateNoStore(parentHandlers.POST);
