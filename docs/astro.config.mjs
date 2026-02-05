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
              "mockd - A fast, lightweight API mocking server for development and testing. Supports HTTP, WebSocket, GraphQL, gRPC, MQTT, and SOAP.",
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
It supports HTTP, WebSocket, GraphQL, gRPC, MQTT, and SOAP protocols.
Features include request matching, response templating, stateful CRUD simulation,
proxy recording, and multi-protocol mock configuration.`,
        }),
      ],
    }),
  ],
  vite: {
    plugins: [tailwindcss()],
  },
});
