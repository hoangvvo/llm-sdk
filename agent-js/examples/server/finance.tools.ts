import { zodTool } from "@hoangvvo/llm-agent/zod";
import z from "zod";

export const getStockPriceTool = zodTool({
  name: "get_stock_price",
  description: "Get current or historical stock price information",
  parameters: z.object({
    symbol: z.string().describe("Stock ticker symbol"),
  }),
  execute: async (input) => {
    const { symbol } = input;

    try {
      const url = `https://query1.finance.yahoo.com/v8/finance/chart/${symbol}`;
      const response = await fetch(url);

      if (!response.ok) {
        throw new Error(`Failed to get stock price for ${symbol}`);
      }

      const data = await response.json();
      const quote = data.chart.result[0];
      const meta = quote.meta;
      const price = meta.regularMarketPrice;

      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(
              {
                symbol,
                price,
                open: meta.regularMarketOpen,
                high: meta.regularMarketDayHigh,
                low: meta.regularMarketDayLow,
                previous_close: meta.previousClose,
                timestamp: new Date(
                  meta.regularMarketTime * 1000,
                ).toISOString(),
              },
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
