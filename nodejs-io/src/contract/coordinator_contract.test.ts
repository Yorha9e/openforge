import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  CreateAgentRequestSchema,
  CreateAgentResponseSchema,
  type CreateAgentRequest,
  type CreateAgentResponse,
} from "../gen/agent/v1/coordinator_pb";
import { loadGolden } from "./golden_loader.js";

describe("Coordinator Contract (CreateAgent)", () => {
  it("CreateAgentRequest maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("coordinator_create_agent.req.json") as {
      agent_id: string;
      pipeline_id: string;
      project_id: string;
      role: string;
      metadata: Record<string, string>;
    };

    const req: CreateAgentRequest = create(CreateAgentRequestSchema, {
      agentId: golden.agent_id,
      pipelineId: golden.pipeline_id,
      projectId: golden.project_id,
      role: golden.role as CreateAgentRequest["role"],
      metadata: golden.metadata,
    });

    expect(req.agentId).toBe("agent-001");
    expect(req.pipelineId).toBe("p-001");
    expect(req.projectId).toBe("proj-1");
    expect(req.role).toBe("AGENT_ROLE_WORKER");
    expect(req.metadata.env).toBe("dev");
    expect(req.metadata.owner).toBe("team-a");
  });

  it("CreateAgentResponse maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("coordinator_create_agent.resp.json") as {
      agent_id: string;
      status: string;
    };

    const resp: CreateAgentResponse = create(CreateAgentResponseSchema, {
      agentId: golden.agent_id,
      status: golden.status,
    });

    expect(resp.agentId).toBe("agent-001");
    expect(resp.status).toBe("RUNNING");
  });
});
