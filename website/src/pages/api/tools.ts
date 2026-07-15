import type { APIRoute } from "astro";
import { listTools } from "../../server/example-api.ts";

export const prerender = false;

export const GET = (() => listTools()) satisfies APIRoute;
