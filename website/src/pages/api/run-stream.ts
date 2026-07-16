import type { APIRoute } from "astro";
import { runStream } from "../../server/example-api.ts";

export const prerender = false;

export const POST = (({ request }) => runStream(request)) satisfies APIRoute;
