export type FailureCode =
  | 'MODEL_HALLUCINATION'
  | 'PROMPT_WEAKNESS'
  | 'DEPENDENCY_CONFLICT'
  | 'SANDBOX_TIMEOUT'
  | 'REPO_BUG'
  | 'CONTEXT_OVERFLOW'
  | 'TOKEN_QUOTA_EXCEEDED'
  | 'UNKNOWN';

export type FailureSeverity = 'warn' | 'error';

export const FAILURE_CODES: readonly FailureCode[] = [
  'MODEL_HALLUCINATION',
  'PROMPT_WEAKNESS',
  'DEPENDENCY_CONFLICT',
  'SANDBOX_TIMEOUT',
  'REPO_BUG',
  'CONTEXT_OVERFLOW',
  'TOKEN_QUOTA_EXCEEDED',
  'UNKNOWN',
] as const;

interface FailureMeta {
  title: string;
  advice: string;
  severity: FailureSeverity;
}

const MESSAGES: Record<FailureCode, FailureMeta> = {
  MODEL_HALLUCINATION: {
    title: 'Model generated inconsistent output',
    advice: 'Try rephrasing the requirement or switching model in Settings.',
    severity: 'warn',
  },
  PROMPT_WEAKNESS: {
    title: 'Prompt needs refinement',
    advice: 'Add more context or constraints; consider decompose stage.',
    severity: 'warn',
  },
  DEPENDENCY_CONFLICT: {
    title: 'Package version conflict',
    advice: 'Update package.json to compatible versions or use legacy-peer-deps.',
    severity: 'error',
  },
  SANDBOX_TIMEOUT: {
    title: 'Sandbox execution exceeded 60s',
    advice: 'Break the task into smaller steps or increase sandbox timeout.',
    severity: 'error',
  },
  REPO_BUG: {
    title: 'Repository state invalid',
    advice: 'Check git status, branch protection rules, and recent commits.',
    severity: 'error',
  },
  CONTEXT_OVERFLOW: {
    title: 'Context window exceeded',
    advice: 'Start a new branch or summarize earlier messages.',
    severity: 'warn',
  },
  TOKEN_QUOTA_EXCEEDED: {
    title: 'Monthly quota exhausted',
    advice: 'Wait until next month or contact your admin to increase budget.',
    severity: 'error',
  },
  UNKNOWN: {
    title: 'Unexpected error',
    advice: 'Try again or contact support with the error ID.',
    severity: 'warn',
  },
};

function isFailureCode(code: string): code is FailureCode {
  return (FAILURE_CODES as readonly string[]).includes(code);
}

export function failureCardTitle(code: string): string {
  if (isFailureCode(code)) return MESSAGES[code].title;
  return MESSAGES.UNKNOWN.title;
}

export function failureCardAdvice(code: string): string {
  if (isFailureCode(code)) return MESSAGES[code].advice;
  return MESSAGES.UNKNOWN.advice;
}

export function failureCardSeverity(code: string): FailureSeverity {
  if (isFailureCode(code)) return MESSAGES[code].severity;
  return MESSAGES.UNKNOWN.severity;
}

export interface FailureCardProps {
  failureCode: string;
  rawMessage?: string;
  errorId?: string;
}

export function FailureCard({ failureCode, rawMessage, errorId }: FailureCardProps) {
  const title = failureCardTitle(failureCode);
  const advice = failureCardAdvice(failureCode);
  const severity = failureCardSeverity(failureCode);
  const color = severity === 'error'
    ? 'border-red-500 bg-red-50'
    : 'border-amber-500 bg-amber-50';
  return (
    <div
      data-testid="failure-card"
      data-failure-code={failureCode}
      className={`rounded-md border-l-4 p-3 my-2 ${color}`}
    >
      <div className="flex items-center gap-2 font-semibold">
        <span aria-hidden="true">!</span>
        {title}
      </div>
      <div className="text-sm text-gray-700 mt-1">{advice}</div>
      {rawMessage && (
        <details className="text-xs text-gray-500 mt-2">
          <summary>Raw error</summary>
          <code className="block mt-1 whitespace-pre-wrap">{rawMessage}</code>
        </details>
      )}
      {errorId && <div className="text-xs text-gray-400 mt-1">Error ID: {errorId}</div>}
    </div>
  );
}
