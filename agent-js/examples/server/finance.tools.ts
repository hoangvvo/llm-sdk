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

export const getCryptoPriceTool = zodTool({
  name: "get_crypto_price",
  description: "Get cryptocurrency price and market information",
  parameters: z.object({
    symbol: z
      .string()
      .describe("Cryptocurrency symbol (e.g., bitcoin, ethereum)"),
    currency: z.string().default("usd").describe("Target currency for price"),
    include_market_data: z
      .boolean()
      .default(true)
      .describe("Include market cap, volume, and price changes"),
  }),
  execute: async (input) => {
    const { symbol, currency, include_market_data } = input;

    try {
      const url = new URL("https://api.coingecko.com/api/v3/simple/price");

      url.searchParams.append("ids", symbol.toLowerCase());
      url.searchParams.append("vs_currencies", currency.toLowerCase());

      if (include_market_data) {
        url.searchParams.append("include_market_cap", "true");
        url.searchParams.append("include_24hr_vol", "true");
        url.searchParams.append("include_24hr_change", "true");
      }
      url.searchParams.append("include_last_updated_at", "true");

      const response = await fetch(url.toString());

      if (!response.ok) {
        throw new Error("Failed to get crypto price");
      }

      const data = await response.json();

      if (!data[symbol.toLowerCase()]) {
        throw new Error(`Cryptocurrency ${symbol} not found`);
      }

      const cryptoData = data[symbol.toLowerCase()];

      const result: {
        symbol: string;
        price: number;
        currency: string;
        last_updated: string | null;
        market_cap?: number;
        "24h_volume"?: number;
        "24h_change_percent"?: number;
      } = {
        symbol,
        price: cryptoData[currency.toLowerCase()],
        currency: currency.toUpperCase(),
        last_updated: cryptoData.last_updated_at
          ? new Date(cryptoData.last_updated_at * 1000).toISOString()
          : null,
      };

      if (include_market_data) {
        result.market_cap = cryptoData[`${currency.toLowerCase()}_market_cap`];
        result["24h_volume"] = cryptoData[`${currency.toLowerCase()}_24h_vol`];
        result["24h_change_percent"] =
          cryptoData[`${currency.toLowerCase()}_24h_change`];
      }

      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(result, null, 2),
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
