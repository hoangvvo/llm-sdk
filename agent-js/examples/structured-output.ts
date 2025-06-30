import { Agent, getContentText, tool } from "@hoangvvo/llm-agent";
import type { ResponseFormatOption } from "@hoangvvo/llm-sdk";
import { getModel } from "./get-model.ts";

// Define the model to use for the Agent
const model = getModel("openai", "gpt-4o");

const searchFlightsTool = tool({
  name: "search_flights",
  description: "Search for flights between two cities",
  parameters: {
    type: "object",
    properties: {
      from: { type: "string", description: "Origin city/airport" },
      to: { type: "string", description: "Destination city/airport" },
      date: { type: "string", description: "Departure date in YYYY-MM-DD" },
    },
    required: ["from", "to", "date"],
    additionalProperties: false,
  },
  execute(args: { from: string; to: string; date: string }) {
    const { from, to, date } = args;
    console.log(`Searching flights from ${from} to ${to} on ${date}`);
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify([
            {
              airline: "Vietnam Airlines",
              departure: `${date}T10:00:00`,
              arrival: `${date}T12:00:00`,
              price: 150,
            },
            {
              airline: "Southwest Airlines",
              departure: `${date}T11:00:00`,
              arrival: `${date}T13:00:00`,
              price: 120,
            },
          ]),
        },
      ],
      is_error: false,
    };
  },
});

const searchHotelsTool = tool({
  name: "search_hotels",
  description: "Search for hotels in a city",
  parameters: {
    type: "object",
    properties: {
      city: { type: "string" },
      checkin: {
        type: "string",
        description: "Check-in date in YYYY-MM-DD",
      },
      nights: { type: "number", description: "Number of nights" },
    },
    required: ["city", "checkin", "nights"],
    additionalProperties: false,
  },
  execute(args: { city: string; checkin: string; nights: number }) {
    const { city, checkin, nights } = args;
    console.log(
      `Searching hotels in ${city} from ${checkin} for ${String(nights)} nights`,
    );
    return {
      content: [
        {
          type: "text",
          text: JSON.stringify([
            {
              name: "The Plaza",
              location: city,
              pricePerNight: 150,
              rating: 4.8,
            },
            {
              name: "Hotel Ritz",
              location: city,
              pricePerNight: 200,
              rating: 4.6,
            },
          ]),
        },
      ],
      is_error: false,
    };
  },
});

// Define the response format
const responseFormat: ResponseFormatOption = {
  type: "json",
  name: "travel_plan",
  description:
    "A structured travel plan including flights, hotels, and weather forecast.",
  schema: {
    type: "object",
    properties: {
      destination: { type: "string" },
      flights: {
        type: "array",
        items: {
          type: "object",
          properties: {
            airline: { type: "string" },
            departure: { type: "string" },
            arrival: { type: "string" },
            price: { type: "number" },
          },
          required: ["airline", "departure", "arrival", "price"],
          additionalProperties: false,
        },
      },
      hotels: {
        type: "array",
        items: {
          type: "object",
          properties: {
            name: { type: "string" },
            location: { type: "string" },
            pricePerNight: { type: "number" },
            rating: { type: "number" },
          },
          required: ["name", "location", "pricePerNight", "rating"],
          additionalProperties: false,
        },
      },
    },
    required: ["destination", "flights", "hotels"],
    additionalProperties: false,
  },
};

const travelAgent = new Agent({
  name: "Bob",
  instructions: [
    "You are Bob, a travel agent that helps users plan their trips.",
    () => `The current time is ${new Date().toISOString()}`,
  ],
  model,
  response_format: responseFormat,
  tools: [searchFlightsTool, searchHotelsTool],
});

const prompt = "Plan a trip from Paris to Tokyo next week";

const response = await travelAgent.run({
  input: [
    {
      type: "message",
      role: "user",
      content: [{ type: "text", text: prompt }],
    },
  ],
  context: {},
});

console.dir(JSON.parse(getContentText(response)));
