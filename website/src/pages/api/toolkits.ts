import type { APIRoute } from "astro";
import { listToolkits } from "../../server/example-api.ts";

export const prerender = false;

export const GET = (() => listToolkits()) satisfies APIRoute;
