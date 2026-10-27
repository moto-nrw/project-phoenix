import { handlers } from "~/server/auth";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const GET = withPrivateNoStore(handlers.GET);
export const POST = withPrivateNoStore(handlers.POST);
