import type { AgentRequest } from "../../src/types.ts";
import type { MyContext } from "./context.ts";

export interface RunStreamBody {
  provider: string;
  model_id: string;
  input: AgentRequest<MyContext>;
  enabled_tools?: string[];
  disabled_instructions?: boolean;
  temperature?: number;
  top_p?: number;
  top_k?: number;
  frequency_penalty?: number;
  presence_penalty?: number;
}
