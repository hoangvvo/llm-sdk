export interface MyContext {
  name?: string;
  location?: string;
  language?: string;
  geo_api_key?: string;
  tomorrow_api_key?: string;
  news_api_key?: string;
  // Client-managed artifacts store (server reads only)
  artifacts?: {
    id: string;
    title: string;
    kind: "markdown" | "text" | "code";
    content: string;
    version?: number;
    updated_at?: string;
  }[];
}
