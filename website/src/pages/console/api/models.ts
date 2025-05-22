import { getModelList } from "../../../../../agent-js/examples/get-model.ts";

export async function GET() {
  const modelList = await getModelList();

  return Response.json(modelList);
}
