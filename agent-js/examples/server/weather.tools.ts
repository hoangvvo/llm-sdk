import { zodTool } from "@hoangvvo/llm-agent/zod";
import z from "zod";
import type { MyContext } from "./context.ts";

export const getCoordinatesTool = zodTool({
  name: "get_coordinates",
  description: "Get coordinates (latitude and longitude) from a location name",
  parameters: z.object({
    location: z.string().describe("The location name, e.g. Paris, France"),
  }),
  execute: async (input, context: MyContext) => {
    const { location } = input;

    const apiKey = context.geo_api_key ?? process.env["GEO_API_KEY"];

    if (apiKey === undefined) {
      return {
        content: [
          {
            type: "text",
            text: "API Key not provided. You can also provide the value on the UI with the Context field 'geo_api_key'. Get a free API key at https://geocode.maps.co/",
          },
        ],
        is_error: true,
      };
    }

    const response = await fetch(
      `https://geocode.maps.co/search?q=${encodeURIComponent(location)}&api_key=${apiKey}`,
    );

    if (!response.ok) {
      return {
        content: [
          {
            type: "text",
            text: `Error fetching coordinates: ${String(response.status)} ${response.statusText}`,
          },
        ],
        is_error: true,
      };
    }

    const items = (await response.json()) as {
      lat: string;
      lon: string;
    }[];

    if (!items[0]) {
      return {
        content: [
          {
            type: "text",
            text: `No coordinates found for location: ${location}`,
          },
        ],
        is_error: true,
      };
    }

    const { lat, lon } = items[0];

    return {
      content: [
        {
          type: "text",
          text: JSON.stringify({ latitude: lat, longitude: lon }),
        },
      ],
      is_error: false,
    };
  },
});

export const getWeatherTool = zodTool({
  name: "get_weather",
  description: "Get current weather from latitude and longitude",
  parameters: z.object({
    latitude: z.string().describe("The latitude"),
    longitude: z.string().describe("The longitude"),
    units: z.enum(["metric", "imperial"]).describe("Units"),
    timesteps: z.enum(["current", "1h", "1d"]).describe("Timesteps"),
    startTime: z.string().describe("Start time in ISO format"),
  }),
  execute: async (input, context: MyContext) => {
    const { latitude, longitude, units, timesteps, startTime } = input;

    const apiKey = context.tomorrow_api_key ?? process.env["TOMORROW_API_KEY"];

    if (apiKey === undefined) {
      return {
        content: [
          {
            type: "text",
            text: "API Key not provided. You can also provide the value on the UI with the Context field 'tomorrow_api_key'. Get a free API key at https://tomorrow.io/",
          },
        ],
        is_error: true,
      };
    }

    const fields = ["temperature", "temperatureApparent", "humidity"].join(",");

    const response = await fetch(
      `https://api.tomorrow.io/v4/timelines?location=${latitude},${longitude}&fields=${fields}&timesteps=${timesteps}&units=${units}&startTime=${startTime}&apikey=${apiKey}`,
    );

    if (!response.ok) {
      return {
        content: [
          {
            type: "text",
            text: `Error fetching weather: ${String(response.status)} ${response.statusText}`,
          },
        ],
        is_error: true,
      };
    }

    const data = await response.json();

    return {
      content: [
        {
          type: "text",
          text: JSON.stringify(data),
        },
      ],
      is_error: false,
    };
  },
});
