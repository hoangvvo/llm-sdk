import { zodTool } from "@hoangvvo/llm-agent/zod";
import z from "zod";
import type { MyContext } from "./context.ts";

export const searchWikipediaTool = zodTool({
  name: "search_wikipedia",
  description: "Search Wikipedia for information on a topic",
  parameters: z.object({
    query: z.string().describe("Search query or article title"),
    language: z.string().describe("Wikipedia language edition").default("en"),
    limit: z
      .number()
      .min(1)
      .max(10)
      .describe("Maximum number of results to return")
      .default(3),
    extract_length: z
      .number()
      .min(50)
      .max(1200)
      .describe("Number of characters for article extract")
      .default(500),
  }),
  execute: async (input) => {
    const { query, language, limit, extract_length } = input;

    try {
      // Search for pages
      const searchUrl = `https://${language}.wikipedia.org/w/api.php`;
      const searchParams = new URLSearchParams({
        action: "opensearch",
        search: query,
        limit: limit.toString(),
        namespace: "0",
        format: "json",
      });

      const searchResponse = await fetch(`${searchUrl}?${searchParams}`);

      if (!searchResponse.ok) {
        throw new Error("Failed to search Wikipedia");
      }

      const searchData = await searchResponse.json();

      if (!searchData[1] || searchData[1].length === 0) {
        return {
          content: [
            {
              type: "text",
              text: JSON.stringify({ results: [], query }),
            },
          ],
          is_error: false,
        };
      }

      // Get extracts for found pages
      const titles = searchData[1].join("|");
      const extractParams = new URLSearchParams({
        action: "query",
        prop: "extracts",
        exintro: "true",
        explaintext: "true",
        exchars: extract_length.toString(),
        titles: titles,
        format: "json",
      });

      const extractResponse = await fetch(`${searchUrl}?${extractParams}`);

      if (!extractResponse.ok) {
        throw new Error(
          `Request failed with status ${String(extractResponse.status)}`,
        );
      }

      const extractData = await extractResponse.json();

      const results: {
        title: string;
        extract: string;
        url: string;
      }[] = [];

      const pages = extractData.query?.pages ?? {};

      for (const pageId in pages) {
        if (pageId !== "-1") {
          const page = pages[pageId];
          results.push({
            title: page.title,
            extract: page.extract ?? "",
            url: `https://${language}.wikipedia.org/wiki/${String(page.title).replace(/ /g, "_")}`,
          });
        }
      }

      return {
        content: [
          {
            type: "text",
            text: JSON.stringify({ results, query }, null, 2),
          },
        ],
        is_error: false,
      };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error: ${error instanceof Error ? error.message : "Unknown error"}`,
          },
        ],
        is_error: true,
      };
    }
  },
});

export const getNewsTool = zodTool({
  name: "get_news",
  description: "Get current news articles based on search criteria",
  parameters: z.object({
    query: z
      .string()
      .max(500)
      .optional()
      .describe("Keywords or phrases to search for"),
    category: z
      .enum([
        "business",
        "entertainment",
        "general",
        "health",
        "science",
        "sports",
        "technology",
      ])
      .default("general")
      .describe("News category filter"),
    country: z.string().optional().describe("ISO 2-letter country code"),
    language: z.string().default("en").describe("ISO 2-letter language code"),
    sort_by: z
      .enum(["relevancy", "popularity", "publishedAt"])
      .default("publishedAt")
      .describe("Sort order for results"),
    limit: z
      .number()
      .min(1)
      .max(100)
      .default(5)
      .describe("Number of articles to return"),
  }),
  execute: async (input, context: MyContext) => {
    const { query, category, country, language, sort_by, limit } = input;
    const { news_api_key } = context;

    try {
      const apiKey = news_api_key ?? process.env["NEWS_API_KEY"];

      if (!apiKey) {
        throw new Error("API key required. Get one free at newsapi.org");
      }

      const baseUrl = "https://newsapi.org/v2/";
      const headers = {
        "X-Api-Key": apiKey,
      };

      let endpoint: string;
      let params: Record<string, string>;

      if (query) {
        endpoint = "everything";
        params = {
          q: query,
          language: language,
          sortBy: sort_by,
          pageSize: limit.toString(),
        };
      } else {
        endpoint = "top-headlines";
        params = {
          category: category,
          country: country ?? "us",
          pageSize: limit.toString(),
        };
      }

      const url = new URL(`${baseUrl}${endpoint}`);
      Object.entries(params).forEach(([key, value]) => {
        url.searchParams.append(key, value);
      });

      const response = await fetch(url.toString(), { headers });

      if (!response.ok) {
        throw new Error(
          `Request failed with status ${String(response.status)}`,
        );
      }

      const data = await response.json();

      const articles = data.articles
        .slice(0, limit)
        .map(
          (article: {
            title: string;
            description: string;
            url: string;
            source: { name: string } | null;
            publishedAt: string;
            author: string;
          }) => ({
            title: article.title,
            description: article.description,
            url: article.url,
            source: article.source?.name,
            published_at: article.publishedAt,
            author: article.author,
          }),
        );

      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(
              { articles, total_results: data.totalResults },
              null,
              2,
            ),
          },
        ],
        is_error: false,
      };
    } catch (error) {
      return {
        content: [
          {
            type: "text",
            text: `Error: ${error instanceof Error ? error.message : "Unknown error"}`,
          },
        ],
        is_error: true,
      };
    }
  },
});
