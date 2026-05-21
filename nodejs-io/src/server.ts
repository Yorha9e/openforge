// server.ts — gRPC server entry point (Node.js IO layer)
//
// Uses @connectrpc/connect v2 with plain Node.js http module.
// Service descriptor comes from protoc-gen-es v2 (llm_pb.ts), NOT from
// llm_connect.ts (generated for connect v1, incompatible with v2 runtime).

import { createServer } from "node:http";
import { connectNodeAdapter } from "@connectrpc/connect-node";
import { create } from "@bufbuild/protobuf";

import { AnthropicProvider } from "./llm/providers/anthropic.js";
import { TokenMeter } from "./llm/token_meter.js";

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

const apiKey = process.env.ANTHROPIC_API_KEY;
if (!apiKey) {
  console.error("FATAL: ANTHROPIC_API_KEY environment variable not set");
  process.exit(1);
}

const anthropic = new AnthropicProvider(apiKey);
const tokenMeter = new TokenMeter();
tokenMeter.start();

// ---------------------------------------------------------------------------
// Router + HTTP server
// ---------------------------------------------------------------------------

const port = parseInt(process.env.PORT || "50051", 10);

const handler = connectNodeAdapter({
  routes(router) {
    // GenService<...> is structurally compatible with DescService;
    // the branded brandv2 fields differ only at the type level.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    router.service(LLMRouterService as any, {
      // ---- Chat (unary) ----------------------------------------------------
      chat: async (req: any) => {
        const messages = req.messages.map((m: any) => ({
          role: m.role,
          content: m.content.map((b: any) => b.text ?? "").join(""),
        }));

        const result = await anthropic.chat({
          messages,
          config: {
            provider: req.config?.provider ?? "anthropic",
            model: req.config?.model ?? "claude-sonnet-4-6",
            apiKey,
            maxTokens: req.config?.maxTokens ?? 4096,
            temperature: req.config?.temperature,
          },
        });

        tokenMeter.record({
          pipelineId: req.pipelineId,
          projectId: "",
          provider: "anthropic",
          model: req.config?.model ?? "claude-sonnet-4-6",
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
          stopReason: "end_turn",
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

        for await (const delta of anthropic.chatStream({
          messages,
          config: {
            provider: req.config?.provider ?? "anthropic",
            model: req.config?.model ?? "claude-sonnet-4-6",
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

      // ---- ListModels (unary) ----------------------------------------------
      listModels: async () => {
        return create(ListModelsResponseSchema, {
          models: [
            create(ModelInfoSchema, {
              provider: "anthropic",
              modelId: "claude-sonnet-4-6",
              displayName: "Claude Sonnet 4.6",
              contextWindow: BigInt(200000),
              inputCostPer1k: 3.0,
              outputCostPer1k: 15.0,
              supportsToolUse: true,
              supportsStreaming: true,
            }),
            create(ModelInfoSchema, {
              provider: "anthropic",
              modelId: "claude-haiku-4-5-20251001",
              displayName: "Claude Haiku 4.5",
              contextWindow: BigInt(200000),
              inputCostPer1k: 0.8,
              outputCostPer1k: 4.0,
              supportsToolUse: true,
              supportsStreaming: true,
            }),
          ],
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
  console.log(`Provider: anthropic | Model: claude-sonnet-4-6`);
});
