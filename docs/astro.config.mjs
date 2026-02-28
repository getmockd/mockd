// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import tailwindcss from "@tailwindcss/vite";
import node from "@astrojs/node";
import starlightLlmsTxt from "starlight-llms-txt";

const ogUrl = new URL("og.png", "https://docs.mockd.io/").href;
const ogImageAlt = "mockd - Fast, lightweight API mocking";

// https://astro.build/config
export default defineConfig({
  site: "https://docs.mockd.io",
  adapter: node({ mode: "standalone" }),
  integrations: [
    starlight({
      title: "mockd",
      description: "Fast, lightweight API mocking for development and testing",
      logo: {
        src: "./src/assets/mockd-logo.svg",
        alt: "mockd",
      },
      favicon: "/favicon.png",
      customCss: ["./src/styles/custom.css"],
      head: [
        {
          tag: "meta",
          attrs: { property: "og:image", content: ogUrl },
        },
        {
          tag: "meta",
          attrs: { property: "og:image:alt", content: ogImageAlt },
        },
        {
          tag: "meta",
          attrs: {
            name: "description",
            content:
              "mockd - A fast, lightweight API mocking server for development and testing. Supports HTTP, WebSocket, GraphQL, gRPC, MQTT, SSE, and SOAP.",
          },
        },
      ],
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/getmockd/mockd",
        },
      ],
      sidebar: [
        {
          label: "Getting Started",
          autogenerate: { directory: "getting-started" },
        },
        {
          label: "Guides",
          collapsed: false,
          autogenerate: { directory: "guides" },
        },
        {
          label: "Protocols",
          collapsed: false,
          autogenerate: { directory: "protocols" },
        },
        {
          label: "Reference",
          collapsed: true,
          autogenerate: { directory: "reference" },
        },
        {
          label: "Examples",
          collapsed: true,
          autogenerate: { directory: "examples" },
        },
      ],
      plugins: [
        starlightLlmsTxt({
          projectName: "mockd",
          description: `mockd is a fast, lightweight API mocking server written in Go.
It supports HTTP, WebSocket, GraphQL, gRPC, MQTT, SSE, and SOAP protocols with built-in OAuth mock provider.
Features include 18 MCP tools for AI agent integration, 35 faker types for response templating,
chaos engineering with 12 fault types (including stateful circuit breakers), deterministic seeded responses,
import from 8 formats (OpenAPI, Postman, HAR, WireMock, cURL, WSDL, Mockoon, mockd),
stateful CRUD simulation, proxy recording, and multi-engine fan-out architecture.`,
        }),
      ],
    }),
  ],
  vite: {
    plugins: [tailwindcss()],
  },
});
