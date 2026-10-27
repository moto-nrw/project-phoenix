import { traegerHandlers } from "~/server/auth/traeger";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const GET = withPrivateNoStore(traegerHandlers.GET);
export const POST = withPrivateNoStore(traegerHandlers.POST);
