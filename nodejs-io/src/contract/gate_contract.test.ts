import { describe, it, expect } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  GateApproveRequestSchema,
  GateApproveResponseSchema,
  type GateApproveRequest,
  type GateApproveResponse,
} from "../gen/agent/v1/gate_pb";
import { loadGolden } from "./golden_loader.js";

describe("Gate Contract (Approve)", () => {
  it("GateApproveRequest maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("gate_approve.req.json") as {
      pipeline_id: string;
      stage: string;
      actor: string;
      checklist: {
        code_reviewed: boolean;
        security_checked: boolean;
        license_cleared: boolean;
        coding_standard_met: boolean;
      };
    };

    const req: GateApproveRequest = create(GateApproveRequestSchema, {
      pipelineId: golden.pipeline_id,
      stage: golden.stage as GateApproveRequest["stage"],
      actor: golden.actor,
      checklist: {
        codeReviewed: golden.checklist.code_reviewed,
        securityChecked: golden.checklist.security_checked,
        licenseCleared: golden.checklist.license_cleared,
        codingStandardMet: golden.checklist.coding_standard_met,
      },
    });

    expect(req.pipelineId).toBe("p-001");
    expect(req.stage).toBe("STAGE_TYPE_IMPL");
    expect(req.actor).toBe("reviewer@example.com");
    expect(req.checklist?.codeReviewed).toBe(true);
    expect(req.checklist?.securityChecked).toBe(true);
    expect(req.checklist?.licenseCleared).toBe(true);
    expect(req.checklist?.codingStandardMet).toBe(true);
  });

  it("GateApproveResponse maps golden fields onto camelCase logical fields", () => {
    const golden = loadGolden("gate_approve.resp.json") as {
      success: boolean;
      next_status: string;
    };

    const resp: GateApproveResponse = create(GateApproveResponseSchema, {
      success: golden.success,
      nextStatus: golden.next_status as GateApproveResponse["nextStatus"],
    });

    expect(resp.success).toBe(true);
    expect(resp.nextStatus).toBe("PIPELINE_STATUS_RUNNING");
  });
});
