// server.ts — gRPC server entry point (Node.js IO layer)
//
// Uses @connectrpc/connect v2 with plain Node.js http module.
// Model names and capabilities are read from environment variables;
// context-window suffixes like "[1m]" are parsed automatically.

import { createServer } from "node:http";
import { connectNodeAdapter } from "@connectrpc/connect-node";
import { create } from "@bufbuild/protobuf";

import { AnthropicProvider } from "./llm/providers/anthropic.js";
import { DeepSeekProvider } from "./llm/providers/deepseek.js";
import { OpenAIProvider } from "./llm/providers/openai.js";
import { TokenMeter } from "./llm/token_meter.js";
import { ModelSelector } from "./llm/domain/model_selector.js";

/** Strip "[Nm]" / "[Nk]" suffix from a model name before sending to the API. */
function stripSuffix(model: string): string {
  return model.replace(/\[\d+[mk]\]$/i, "");
}

// Generated proto types and service descriptor (protoc-gen-es v2 GenService)
import {
  LLMRouterService,
  LLMChatResponseSchema,
  LLMContentBlockSchema,
  LLMUsageSchema,
  LLMChatStreamChunkSchema,
  GetTokenUsageResponseSchema,
  ListModelsResponseSchema,
  ModelInfoSchema,
  SwitchModelResponseSchema,
} from "./gen/agent/v1/llm_pb.js";

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

const apiKey = process.env.ANTHROPIC_API_KEY || process.env.ANTHROPIC_AUTH_TOKEN;
if (!apiKey) {
  console.error("FATAL: ANTHROPIC_API_KEY or ANTHROPIC_AUTH_TOKEN must be set");
  process.exit(1);
}

const baseURL = process.env.ANTHROPIC_BASE_URL;
if (baseURL) {
  console.log(`Base URL: ${baseURL}`);
}

const anthropic = new AnthropicProvider(apiKey, baseURL);
const deepseek = new DeepSeekProvider(
  process.env.DEEPSEEK_BASE_URL ?? "https://api.deepseek.com/anthropic",
  process.env.DEEPSEEK_API_KEY ?? apiKey,
);
const openai = new OpenAIProvider(
  process.env.OPENAI_BASE_URL ?? "https://api.openai.com",
  process.env.OPENAI_API_KEY ?? "",
);
const tokenMeter = new TokenMeter();
tokenMeter.start();

// Model selection from env vars (see model_selector.ts for suffix parsing).
// Env vars follow the Anthropic-compatible template:
//   ANTHROPIC_MODEL                — default model
//   ANTHROPIC_DEFAULT_SONNET_MODEL — Sonnet-tier model (priority 0)
//   ANTHROPIC_DEFAULT_OPUS_MODEL   — Opus-tier model   (priority 1)
//   ANTHROPIC_DEFAULT_HAIKU_MODEL  — Haiku-tier model  (priority 2)
//
// Each model name may carry a context-window suffix: "mimo-v2.5-pro[1m]".
const modelSelector = new ModelSelector();
const defaultModel = process.env.ANTHROPIC_MODEL || modelSelector.select()!.model;
const defaultModelCtx = modelSelector.select(defaultModel)?.contextWindow ?? 200_000;

console.log(`Models: ${modelSelector.list().map((m) => `${m.model}[${m.contextWindow.toLocaleString()}]`).join(", ")}`);
console.log(`Default: ${defaultModel} (${defaultModelCtx.toLocaleString()} ctx)`);

// ---------------------------------------------------------------------------
// Router + HTTP server
// ---------------------------------------------------------------------------

const port = parseInt(process.env.PORT || "50051", 10);

const handler = connectNodeAdapter({
  routes(router) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    router.service(LLMRouterService as any, {
      // ---- Chat (unary) ----------------------------------------------------
      chat: async (req: any) => {
        const messages = req.messages.map((m: any) => ({
          role: m.role,
          content: m.content.map((b: any) => b.text ?? "").join(""),
        }));

        // Extract tools from proto request
        const tools = (req.tools ?? []).map((t: any) => ({
          name: t.name,
          description: t.description,
          inputSchema: t.inputSchema
            ? JSON.parse(new TextDecoder().decode(t.inputSchema))
            : {},
        }));

        const model = stripSuffix(req.config?.model || defaultModel);

        const result = await anthropic.chat({
          messages,
          tools,
          config: {
            provider: req.config?.provider ?? "anthropic",
            model,
            apiKey,
            maxTokens: req.config?.maxTokens ?? 4096,
            temperature: req.config?.temperature,
          },
        });

        tokenMeter.record({
          pipelineId: req.pipelineId,
          projectId: "",
          provider: "anthropic",
          model,
          inputTokens: result.usage.inputTokens,
          outputTokens: result.usage.outputTokens,
          timestamp: new Date(),
        });

        return create(LLMChatResponseSchema, {
          id: result.id,
          content: [
            create(LLMContentBlockSchema, {
              type: "text",
              text: result.content,
            }),
          ],
          stopReason: result.stopReason ?? "end_turn",
          usage: create(LLMUsageSchema, {
            inputTokens: BigInt(result.usage.inputTokens),
            outputTokens: BigInt(result.usage.outputTokens),
          }),
        });
      },

      // ---- ChatStream (server-streaming) -----------------------------------
      chatStream: async function* (req: any) {
        const messages = req.messages.map((m: any) => ({
          role: m.role,
          content: m.content.map((b: any) => b.text ?? "").join(""),
        }));

        const model = stripSuffix(req.config?.model || defaultModel);

        for await (const delta of anthropic.chatStream({
          messages,
          config: {
            provider: req.config?.provider ?? "anthropic",
            model,
            apiKey,
            maxTokens: req.config?.maxTokens ?? 4096,
            temperature: req.config?.temperature,
          },
        })) {
          yield create(LLMChatStreamChunkSchema, {
            eventType: "delta",
            delta: create(LLMContentBlockSchema, {
              type: "text_delta",
              text: delta,
            }),
          });
        }

        yield create(LLMChatStreamChunkSchema, {
          eventType: "done",
        });
      },

      // ---- ListModels (unary) — dynamic from env vars ----------------------
      listModels: async () => {
        const entries = modelSelector.list();
        return create(ListModelsResponseSchema, {
          models: entries.map((e) =>
            create(ModelInfoSchema, {
              provider: e.provider,
              modelId: e.model,
              displayName: `${e.provider}/${e.model}`,
              contextWindow: BigInt(e.contextWindow),
              inputCostPer1k: 0,
              outputCostPer1k: 0,
              supportsToolUse: true,
              supportsStreaming: true,
            }),
          ),
        });
      },

      // ---- SwitchModel (unary) ---------------------------------------------
      switchModel: async (req: any) => {
        return create(SwitchModelResponseSchema, {
          success: true,
          activeConfig: req.newConfig,
          message: "Model will take effect on next conversation turn",
        });
      },

      // ---- GetTokenUsage (unary) -------------------------------------------
      getTokenUsage: async () => {
        return create(GetTokenUsageResponseSchema, {
          totalInputTokens: BigInt(0),
          totalOutputTokens: BigInt(0),
          totalCost: 0,
          byProvider: [],
        });
      },
    });
  },
});

const server = createServer(handler);

server.listen(port, "0.0.0.0", () => {
  console.log(`OpenForge Node.js IO layer listening on :${port}`);
});
