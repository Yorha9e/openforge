// server.ts — gRPC server entry point (Node.js IO layer)
//
// Uses @connectrpc/connect v2 with plain Node.js http module.
// Model names and capabilities are read from environment variables;
// context-window suffixes like "[1m]" are parsed automatically.

import { createServer } from "node:http";
import { randomUUID } from "node:crypto";
import { connectNodeAdapter, createConnectTransport } from "@connectrpc/connect-node";
import { createClient, type Client } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";

// Path C T7: OpenTelemetry Node SDK + W3C traceparent propagation.
// Must be imported BEFORE any instrumented module so the auto-instrumentation
// can patch http, connectrpc, etc.  The OTLP/gRPC exporter sends spans to
// the same collector the Go server exports to, allowing the full Go↔Node
// trace to be assembled in a single backend.
import { NodeSDK } from "@opentelemetry/sdk-node";
import { OTLPTraceExporter } from "@opentelemetry/exporter-trace-otlp-grpc";
import { resourceFromAttributes } from "@opentelemetry/resources";
import { ATTR_SERVICE_NAME } from "@opentelemetry/semantic-conventions";
import { HttpInstrumentation } from "@opentelemetry/instrumentation-http";
import { diag, DiagConsoleLogger, DiagLogLevel } from "@opentelemetry/api";

if (process.env.OTEL_DEBUG === "1") {
  diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.INFO);
}

const otlpEndpoint = process.env.OTLP_ENDPOINT ?? "localhost:4317";
const nodeSDK = new NodeSDK({
  resource: resourceFromAttributes({
    [ATTR_SERVICE_NAME]: "openforge-nodejs-io",
  }),
  traceExporter: new OTLPTraceExporter({ url: `http://${otlpEndpoint}` }),
  instrumentations: [
    // Auto-instrument incoming HTTP so traceparent is extracted from the
    // Go client's ConnectRPC requests and outgoing fetch calls (e.g. the
    // token_meter transport) carry an injected traceparent.
    new HttpInstrumentation(),
  ],
});
try {
  nodeSDK.start();
} catch (err) {
  console.error("OTel NodeSDK start failed; continuing without tracing", err);
}

const shutdownOTel = (): void => {
  try {
    nodeSDK.shutdown().catch((err) => {
      console.error("OTel NodeSDK shutdown failed", err);
    });
  } catch (err) {
    console.error("OTel NodeSDK shutdown threw", err);
  }
};
process.on("SIGTERM", shutdownOTel);
process.on("SIGINT", shutdownOTel);

import { AnthropicProvider } from "./llm/providers/anthropic.js";
import { DeepSeekProvider } from "./llm/providers/deepseek.js";
import { OpenAIProvider } from "./llm/providers/openai.js";
import { TokenMeter, type FlushTransport, type TokenRecord } from "./llm/token_meter.js";
import { ModelSelector } from "./llm/domain/model_selector.js";
import type { LLMProvider } from "./kernel/interfaces.js";
import { LLMRouterService } from "./gen/agent/v1/llm_connect.js";
import {
  LLMChatResponseSchema,
  LLMContentBlockSchema,
  LLMUsageSchema,
  LLMChatStreamChunkSchema,
  GetTokenUsageResponseSchema,
  ListModelsResponseSchema,
  ModelInfoSchema,
  SwitchModelResponseSchema,
  RecordTokenUsageRequestSchema,
  TokenUsageRecordSchema,
} from "./gen/agent/v1/llm_pb.js";
import {
  CoordinatorService,
  CreateAgentResponseSchema,
  DestroyAgentResponseSchema,
  ExecuteStageEventSchema,
  ChatEventSchema,
  EditMessageResponseSchema,
  StopGenerationResponseSchema,
  PauseGenerationResponseSchema,
  GetPipelineResponseSchema,
  CancelPipelineResponseSchema,
  ModifyPipelineScopeResponseSchema,
  TokenUsageAckSchema,
  HealthResponseSchema,
} from "./gen/agent/v1/coordinator_pb.js";
import {
  GateService,
  GateApproveResponseSchema,
  GateRejectResponseSchema,
  GateClaimResponseSchema,
  GateGetInboxResponseSchema,
} from "./gen/agent/v1/gate_pb.js";
import {
  ToolRegistryService,
  SearchToolsResponseSchema,
  CallToolResponseSchema,
  CallToolStreamChunkSchema,
  RegisterToolResponseSchema,
  UnregisterToolResponseSchema,
  ListAllToolsResponseSchema,
  RebuildIndexResponseSchema,
  GetIndexStatusResponseSchema,
} from "./gen/agent/v1/tools_pb.js";

/** Strip "[Nm]" / "[Nk]" suffix from a model name before sending to the API. */
function stripSuffix(model: string): string {
  return model.replace(/\[\d+[mk]\]$/i, "");
}

// Generated proto types and service descriptor (protoc-gen-es v2 GenService)
// are imported above from "./gen/agent/v1/llm_connect.js" + "./gen/agent/v1/llm_pb.js".

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

// T2: Wire TokenMeter.flush() → Go gRPC RecordTokenUsage → token_usage table.
// The Go coordinate layer listens on OF_GRPC_ADDR (default :8030) and exposes
// LLMRouterService at /agent.v1.LLMRouterService/. If the env var is unset we
// fall back to the default; failures bubble up to TokenMeter which re-enqueues
// the batch (bounded by 2 * maxBufferSize).
const goGrpcAddr = process.env.OF_GRPC_ADDR ?? "http://127.0.0.1:8030";
const goTransport = createConnectTransport({ baseUrl: goGrpcAddr, httpVersion: "1.1" });
// The generated descriptor uses @bufbuild/protobuf's `as const` shape, which
// is structurally compatible with DescService at runtime. The type system
// in this version of @connectrpc/connect expects a stricter DescService type,
// so we cast through any — the same pattern the connectNodeAdapter uses below.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const goLLMClient = createClient(LLMRouterService as any, goTransport) as any;

const tokenUsageFlush: FlushTransport = async (batch: TokenRecord[]) => {
  await goLLMClient.recordTokenUsage(
    create(RecordTokenUsageRequestSchema, {
      records: batch.map((r) =>
        create(TokenUsageRecordSchema, {
          id: randomUUID(),
          pipelineId: r.pipelineId,
          projectId: r.projectId,
          provider: r.provider,
          model: r.model,
          promptTokens: BigInt(r.inputTokens),
          completionTokens: BigInt(r.outputTokens),
          estimatedCost: 0, // TODO: 接 Anthropic pricing
          createdAt: timestampFromDate(r.timestamp),
        }),
      ),
    }),
  );
};

const tokenMeter = new TokenMeter(tokenUsageFlush);
tokenMeter.start();

const providers: Record<string, LLMProvider> = {
  anthropic,
  deepseek,
  openai,
};

function resolveProvider(name: string): LLMProvider {
  const p = providers[name];
  if (!p) {
    throw new Error(`Unknown LLM provider: ${name}. Available: ${Object.keys(providers).join(", ")}`);
  }
  return p;
}

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

        const providerName = req.config?.provider ?? "anthropic";
        const provider = resolveProvider(providerName);
        const result = await provider.chat({
          messages,
          tools,
          config: {
            provider: providerName,
            model,
            apiKey,
            maxTokens: req.config?.maxTokens ?? 4096,
            temperature: req.config?.temperature,
          },
        });

        tokenMeter.record({
          pipelineId: req.pipelineId,
          projectId: "",
          provider: providerName,
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

        // Estimate input tokens from character count (~4 chars per token)
        const inputChars = messages.reduce(
          (sum: number, m: { content: string }) => sum + m.content.length,
          0,
        );

        const model = stripSuffix(req.config?.model || defaultModel);

        const providerName = req.config?.provider ?? "anthropic";
        const provider = resolveProvider(providerName);
        let outputChars = 0;
        for await (const delta of provider.chatStream({
          messages,
          config: {
            provider: req.config?.provider ?? "anthropic",
            model,
            apiKey,
            maxTokens: req.config?.maxTokens ?? 4096,
            temperature: req.config?.temperature,
          },
        })) {
          outputChars += delta.length;
          yield create(LLMChatStreamChunkSchema, {
            eventType: "delta",
            delta: create(LLMContentBlockSchema, {
              type: "text_delta",
              text: delta,
            }),
          });
        }

        // Record token usage with char-based estimation
        tokenMeter.record({
          pipelineId: req.pipelineId,
          projectId: "",
          provider: providerName,
          model,
          inputTokens: Math.ceil(inputChars / 4),
          outputTokens: Math.ceil(outputChars / 4),
          timestamp: new Date(),
        });

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
        const summary = tokenMeter.getSummary();
        return create(GetTokenUsageResponseSchema, {
          totalInputTokens: summary.totalInputTokens,
          totalOutputTokens: summary.totalOutputTokens,
          totalCost: summary.totalCost,
          byProvider: summary.byProvider.map((p) => ({
            provider: p.provider,
            inputTokens: p.inputTokens,
            outputTokens: p.outputTokens,
            requestCount: BigInt(p.requestCount),
            cost: 0,
          })),
        });
      },
    });

    // Path C T1: Wire CoordinatorService + GateService +
    // ToolRegistryService on the Node.js IO process. Each RPC returns
    // a shape-correct stub — real Node-side business logic for the
    // IO layer is scoped to T2 (Coordinator) and T5 (Gate).
    // ToolRegistryService is owned by the Dynamic Tool Hub (T8).
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    router.service(CoordinatorService as any, {
      createAgent: async (req: any) =>
        create(CreateAgentResponseSchema, {
          agentId: req.agentId,
          status: "stub",
        }),
      destroyAgent: async () =>
        create(DestroyAgentResponseSchema, { success: true }),
      executeStage: async function* () {
        yield create(ExecuteStageEventSchema, { eventType: "done" });
      },
      chat: async function* () {
        yield create(ChatEventSchema, { eventType: 1, isDone: true });
      },
      editMessage: async (req: any) =>
        create(EditMessageResponseSchema, {
          success: true,
          newBranchId: req.branchId + "-stub",
        }),
      stopGeneration: async () =>
        create(StopGenerationResponseSchema, { success: true }),
      pauseGeneration: async () =>
        create(PauseGenerationResponseSchema, {
          success: true,
          checkpointSeq: 0,
        }),
      resumeGeneration: async function* () {
        yield create(ChatEventSchema, { eventType: 1, isDone: true });
      },
      regenerateFrom: async function* () {
        yield create(ChatEventSchema, { eventType: 1, isDone: true });
      },
      getPipeline: async (req: any) =>
        create(GetPipelineResponseSchema, { pipelineId: req.pipelineId }),
      cancelPipeline: async () =>
        create(CancelPipelineResponseSchema, {
          success: true,
          finalStatus: 8,
        }),
      modifyPipelineScope: async () =>
        create(ModifyPipelineScopeResponseSchema, { success: true }),
      pushTokenUsage: async () =>
        create(TokenUsageAckSchema, { success: true }),
      health: async () =>
        create(HealthResponseSchema, { serving: true }),
    });

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    router.service(GateService as any, {
      approve: async () =>
        create(GateApproveResponseSchema, { success: true, nextStatus: 2 }),
      reject: async () =>
        create(GateRejectResponseSchema, { success: true }),
      claim: async () =>
        create(GateClaimResponseSchema, { success: true }),
      getInbox: async () =>
        create(GateGetInboxResponseSchema, { items: [], nextPageToken: "" }),
    });

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    router.service(ToolRegistryService as any, {
      searchTools: async () =>
        create(SearchToolsResponseSchema, { matches: [] }),
      callTool: async (req: any) =>
        create(CallToolResponseSchema, { toolName: req.toolName }),
      callToolStream: async function* () {
        yield create(CallToolStreamChunkSchema, {
          eventType: "done",
          isDone: true,
        });
      },
      registerTool: async (req: any) =>
        create(RegisterToolResponseSchema, {
          success: true,
          toolName: req.tool?.name ?? "",
        }),
      unregisterTool: async () =>
        create(UnregisterToolResponseSchema, { success: true }),
      listAllTools: async () =>
        create(ListAllToolsResponseSchema, { tools: [], totalCount: 0 }),
      rebuildIndex: async () =>
        create(RebuildIndexResponseSchema, { success: true, status: "queued" }),
      getIndexStatus: async () =>
        create(GetIndexStatusResponseSchema, { status: "ready" }),
    });
  },
});

const server = createServer(handler);

server.listen(port, "0.0.0.0", () => {
  console.log(`OpenForge Node.js IO layer listening on :${port}`);
});
