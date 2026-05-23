package domain

import "context"

// GateHook is a pre/post interceptor on gate approval/rejection.
type GateHook interface {
	PreApprove(ctx context.Context, event *GateEvent) error
	PostApprove(ctx context.Context, event *GateEvent)
	PreReject(ctx context.Context, event *GateEvent) error
	PostReject(ctx context.Context, event *GateEvent)
}

// HookChain executes hooks in order, stopping on first error.
type HookChain []GateHook

func (hc HookChain) RunPreApprove(ctx context.Context, ev *GateEvent) error {
	for _, h := range hc {
		if err := h.PreApprove(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (hc HookChain) RunPostApprove(ctx context.Context, ev *GateEvent) {
	for _, h := range hc {
		h.PostApprove(ctx, ev)
	}
}

func (hc HookChain) RunPreReject(ctx context.Context, ev *GateEvent) error {
	for _, h := range hc {
		if err := h.PreReject(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

func (hc HookChain) RunPostReject(ctx context.Context, ev *GateEvent) {
	for _, h := range hc {
		h.PostReject(ctx, ev)
	}
}
