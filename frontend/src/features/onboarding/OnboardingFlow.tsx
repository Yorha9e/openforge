import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { tokens } from '../../shared/design-tokens';

type Role = 'pm' | 'developer' | 'reviewer';
type SetupType = 'existing' | 'new';

interface RoleOption {
  id: Role;
  title: string;
  description: string;
}

const ROLES: RoleOption[] = [
  {
    id: 'pm',
    title: 'PM',
    description: 'Define requirements, review deliverables, and approve pipeline gates. Drive the project forward with AI-assisted planning.',
  },
  {
    id: 'developer',
    title: 'Developer',
    description: 'Write code, review diffs, and collaborate with AI agents to implement features. Full access to the pipeline workspace.',
  },
  {
    id: 'reviewer',
    title: 'Reviewer',
    description: 'Review pipeline output, approve or request changes at each gate. Focus on quality and consistency.',
  },
];

const DEMO_MESSAGES = [
  { role: 'user', text: 'Create a new feature for user authentication' },
  { role: 'assistant', text: "I'll help you build a user authentication feature. Let me create a pipeline to handle this." },
  { role: 'assistant', text: 'Pipeline created. Stage 1 (Clarify) is running — I need to ask a few questions about your requirements.' },
  { role: 'user', text: 'We need JWT-based auth with email + password, and OAuth for Google login.' },
  { role: 'assistant', text: 'Great. I have enough context. Stage 2 (Implement) is starting. I will generate the backend auth service, middleware, and frontend login components.' },
];

function StepIndicator({ currentStep }: { currentStep: number }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 0, marginBottom: 40 }}>
      {[1, 2, 3].map((step) => (
        <div key={step} style={{ display: 'flex', alignItems: 'center' }}>
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: '50%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 14,
              fontWeight: 600,
              fontFamily: tokens.fontHeading,
              background: step < currentStep ? tokens.cta : step === currentStep ? tokens.cta : tokens.border,
              color: step <= currentStep ? tokens.bg : tokens.muted,
              transition: 'background 300ms, color 300ms',
              border: step === currentStep ? `2px solid ${tokens.cta}` : step < currentStep ? `2px solid ${tokens.cta}` : `2px solid ${tokens.border}`,
            }}
          >
            {step < currentStep ? (
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#0F172A" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            ) : (
              step
            )}
          </div>
          {step < 3 && (
            <div
              style={{
                width: 60,
                height: 2,
                background: step < currentStep ? tokens.cta : tokens.border,
                transition: 'background 300ms',
              }}
            />
          )}
        </div>
      ))}
    </div>
  );
}

function StepTitle({ step, title }: { step: number; title: string }) {
  return (
    <div style={{ textAlign: 'center', marginBottom: 32 }}>
      <p style={{ fontSize: 12, color: tokens.muted, fontFamily: tokens.fontHeading, margin: '0 0 8px 0', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
        Step {step} of 3
      </p>
      <h2 style={{ fontSize: 22, fontWeight: 700, color: tokens.text, fontFamily: tokens.fontHeading, margin: 0 }}>
        {title}
      </h2>
    </div>
  );
}

function RoleSelection({ selected, onSelect }: { selected: Role | null; onSelect: (role: Role) => void }) {
  return (
    <>
      <StepTitle step={1} title="Choose Your Role" />
      <p style={{ fontSize: 14, color: tokens.muted, textAlign: 'center', margin: '0 0 24px 0', lineHeight: 1.6 }}>
        Select how you will use OpenForge. This helps us tailor the experience.
      </p>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {ROLES.map((role) => {
          const isActive = selected === role.id;
          return (
            <button
              key={role.id}
              onClick={() => onSelect(role.id)}
              onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') onSelect(role.id); }}
              style={{
                display: 'block',
                width: '100%',
                padding: '16px 20px',
                background: isActive ? tokens.surface : tokens.bg,
                border: isActive ? `1px solid ${tokens.cta}` : `1px solid ${tokens.border}`,
                borderRadius: 8,
                cursor: 'pointer',
                textAlign: 'left',
                transition: 'border-color 200ms, background 200ms',
              }}
              onMouseEnter={(e) => {
                if (!isActive) {
                  e.currentTarget.style.borderColor = tokens.cta;
                  e.currentTarget.style.background = tokens.surface;
                }
              }}
              onMouseLeave={(e) => {
                if (!isActive) {
                  e.currentTarget.style.borderColor = tokens.border;
                  e.currentTarget.style.background = tokens.bg;
                }
              }}
            >
              <h3 style={{ fontSize: 15, fontWeight: 600, color: tokens.text, fontFamily: tokens.fontHeading, margin: '0 0 6px 0' }}>
                {isActive ? '[ ' : ''}{role.title}{isActive ? ' ]' : ''}
              </h3>
              <p style={{ fontSize: 13, color: tokens.muted, margin: 0, lineHeight: 1.5 }}>
                {role.description}
              </p>
            </button>
          );
        })}
      </div>
    </>
  );
}

function ProjectSetup({
  setupType,
  repoUrl,
  error,
  onSetupTypeChange,
  onRepoUrlChange,
}: {
  setupType: SetupType;
  repoUrl: string;
  error: string;
  onSetupTypeChange: (t: SetupType) => void;
  onRepoUrlChange: (url: string) => void;
}) {
  const isValidUrl = repoUrl.length === 0 || /^https?:\/\/.+/.test(repoUrl);

  return (
    <>
      <StepTitle step={2} title="Project Setup" />
      <div style={{ display: 'flex', gap: 8, marginBottom: 20 }}>
        <button
          onClick={() => onSetupTypeChange('existing')}
          style={{
            flex: 1,
            padding: '10px 16px',
            background: setupType === 'existing' ? tokens.surface : tokens.bg,
            border: setupType === 'existing' ? `1px solid ${tokens.cta}` : `1px solid ${tokens.border}`,
            borderRadius: 6,
            color: '#F8FAFC',
            fontSize: 13,
            fontWeight: 500,
            cursor: 'pointer',
            transition: 'border-color 200ms, background 200ms',
          }}
        >
          Connect Existing Repo
        </button>
        <button
          onClick={() => onSetupTypeChange('new')}
          style={{
            flex: 1,
            padding: '10px 16px',
            background: setupType === 'new' ? tokens.surface : tokens.bg,
            border: setupType === 'new' ? `1px solid ${tokens.cta}` : `1px solid ${tokens.border}`,
            borderRadius: 6,
            color: '#F8FAFC',
            fontSize: 13,
            fontWeight: 500,
            cursor: 'pointer',
            transition: 'border-color 200ms, background 200ms',
          }}
        >
          Start Fresh Project
        </button>
      </div>

      {setupType === 'existing' ? (
        <div>
          <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>
            Git Repository URL
          </label>
          <input
            type="url"
            value={repoUrl}
            onChange={(e) => onRepoUrlChange(e.target.value)}
            placeholder="https://github.com/username/repository.git"
            style={{
              width: '100%',
              padding: '10px 14px',
              background: tokens.bg,
              border: error
                ? `1px solid ${tokens.error}`
                : repoUrl.length > 0 && !isValidUrl
                  ? `1px solid ${tokens.error}`
                  : `1px solid ${tokens.border}`,
              borderRadius: 6,
              color: tokens.text,
              fontSize: 14,
              outline: 'none',
              boxSizing: 'border-box',
            }}
            autoFocus
          />
          {error && (
            <p role="alert" style={{ fontSize: 12, color: tokens.error, margin: '6px 0 0 0' }}>{error}</p>
          )}
          {repoUrl.length > 0 && !isValidUrl && !error && (
            <p style={{ fontSize: 12, color: tokens.error, margin: '6px 0 0 0' }}>
              Please enter a valid HTTP or HTTPS URL.
            </p>
          )}
          <p style={{ fontSize: 12, color: tokens.muted, margin: '8px 0 0 0', lineHeight: 1.5 }}>
            OpenForge will clone the repository and analyze its structure. Make sure you have access permissions.
          </p>
        </div>
      ) : (
        <div>
          <div style={{
            padding: 20,
            background: tokens.surface,
            border: `1px solid ${tokens.border}`,
            borderRadius: 8,
            textAlign: 'center',
          }}>
            <div style={{
              width: 48,
              height: 48,
              borderRadius: 12,
              background: `${tokens.cta}20`,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              margin: '0 auto 12px auto',
            }}>
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke={tokens.cta} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
            </div>
            <h3 style={{ fontSize: 15, fontWeight: 600, color: tokens.text, fontFamily: tokens.fontHeading, margin: '0 0 6px 0' }}>
              New Project
            </h3>
            <p style={{ fontSize: 13, color: tokens.muted, margin: 0, lineHeight: 1.5 }}>
              OpenForge will scaffold a new project structure based on your requirements.
            </p>
          </div>
        </div>
      )}
    </>
  );
}

function DemoChat() {
  const [visibleCount, setVisibleCount] = useState(0);

  return (
    <>
      <StepTitle step={3} title="See It in Action" />
      <p style={{ fontSize: 14, color: tokens.muted, textAlign: 'center', margin: '0 0 20px 0', lineHeight: 1.6 }}>
        This is how you will interact with OpenForge. Type requirements, review AI output, and approve at each gate.
      </p>

      <div
        style={{
          background: tokens.bg,
          border: `1px solid ${tokens.border}`,
          borderRadius: 10,
          overflow: 'hidden',
          maxHeight: 360,
          overflowY: 'auto',
        }}
      >
        <div style={{
          padding: '10px 14px',
          borderBottom: `1px solid ${tokens.border}`,
          display: 'flex',
          alignItems: 'center',
          gap: 8,
        }}>
          <div style={{ width: 10, height: 10, borderRadius: '50%', background: tokens.cta }} />
          <span style={{ fontSize: 12, color: tokens.muted, fontFamily: tokens.fontHeading }}>openforge-pipeline</span>
        </div>
        <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
          {DEMO_MESSAGES.map((msg, i) => (
            <div
              key={i}
              style={{
                alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
                maxWidth: '85%',
              }}
            >
              {(!visibleCount || i < visibleCount) ? null : null}
              <div
                style={{
                  padding: '10px 14px',
                  borderRadius: msg.role === 'user' ? '14px 14px 4px 14px' : '14px 14px 14px 4px',
                  background: msg.role === 'user' ? tokens.cta : tokens.surface,
                  color: msg.role === 'user' ? tokens.bg : tokens.text,
                  fontSize: 13,
                  lineHeight: 1.5,
                  border: msg.role === 'assistant' ? `1px solid ${tokens.border}` : 'none',
                }}
              >
                {msg.text}
              </div>
              <p style={{
                fontSize: 11,
                color: tokens.muted,
                margin: '4px 0 0 0',
                textAlign: msg.role === 'user' ? 'right' : 'left',
              }}>
                {msg.role === 'user' ? 'You' : 'OpenForge AI'}
              </p>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

/**
 * 3-step onboarding flow: Role Selection -> Project Setup -> Demo Chat.
 * Shown on first login or via /onboarding route.
 */
export function OnboardingFlow() {
  const navigate = useNavigate();
  const [currentStep, setCurrentStep] = useState(1);
  const [selectedRole, setSelectedRole] = useState<Role | null>(null);
  const [setupType, setSetupType] = useState<SetupType>('existing');
  const [repoUrl, setRepoUrl] = useState('');
  const [error, setError] = useState('');

  const canProceedFromStep1 = selectedRole !== null;
  const canProceedFromStep2 = setupType === 'new' || (setupType === 'existing' && /^https?:\/\/.+/.test(repoUrl));

  const handleNext = () => {
    if (currentStep === 1 && !canProceedFromStep1) return;
    if (currentStep === 2) {
      if (setupType === 'existing' && !/^https?:\/\/.+/.test(repoUrl)) {
        setError('Please enter a valid repository URL.');
        return;
      }
      setError('');
    }
    if (currentStep < 3) {
      setCurrentStep((s) => s + 1);
    }
  };

  const handleBack = () => {
    if (currentStep > 1) {
      setCurrentStep((s) => s - 1);
    }
  };

  const handleSkip = () => {
    navigate('/', { replace: true });
  };

  const handleFinish = () => {
    navigate('/', { replace: true });
  };

  const handleRoleSelect = (role: Role) => {
    setSelectedRole(role);
  };

  const handleSetupTypeChange = (t: SetupType) => {
    setSetupType(t);
    setError('');
  };

  const handleRepoUrlChange = (url: string) => {
    setRepoUrl(url);
    if (error && /^https?:\/\/.+/.test(url)) {
      setError('');
    }
  };

  return (
    <div style={{
      minHeight: '100vh',
      background: tokens.bg,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      padding: 24,
      fontFamily: tokens.fontBody,
    }}>
      <div style={{ width: '100%', maxWidth: 560 }}>
        {/* Header */}
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <h1 style={{
            fontSize: 20,
            fontWeight: 700,
            color: tokens.text,
            fontFamily: tokens.fontHeading,
            margin: '0 0 4px 0',
          }}>
            Welcome to OpenForge
          </h1>
          <p style={{ fontSize: 13, color: tokens.muted, margin: 0 }}>
            AI-Powered Development Workbench
          </p>
        </div>

        <StepIndicator currentStep={currentStep} />

        {/* Main Card */}
        <div style={{
          background: tokens.surface,
          border: `1px solid ${tokens.border}`,
          borderRadius: 12,
          padding: 32,
          marginBottom: 24,
        }}>
          {currentStep === 1 && (
            <RoleSelection selected={selectedRole} onSelect={handleRoleSelect} />
          )}
          {currentStep === 2 && (
            <ProjectSetup
              setupType={setupType}
              repoUrl={repoUrl}
              error={error}
              onSetupTypeChange={handleSetupTypeChange}
              onRepoUrlChange={handleRepoUrlChange}
            />
          )}
          {currentStep === 3 && <DemoChat />}
        </div>

        {/* Navigation */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
        }}>
          <div>
            {currentStep > 1 ? (
              <button
                onClick={handleBack}
                style={{
                  padding: '10px 20px',
                  background: 'transparent',
                  border: `1px solid ${tokens.border}`,
                  borderRadius: 6,
                  color: tokens.muted,
                  fontSize: 13,
                  fontWeight: 500,
                  cursor: 'pointer',
                  transition: 'border-color 200ms, color 200ms',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = tokens.text;
                  e.currentTarget.style.color = tokens.text;
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = tokens.border;
                  e.currentTarget.style.color = tokens.muted;
                }}
              >
                &larr; Back
              </button>
            ) : (
              <div />
            )}
          </div>

          <div style={{ display: 'flex', gap: 8 }}>
            <button
              onClick={handleSkip}
              style={{
                padding: '10px 20px',
                background: 'transparent',
                border: `1px solid ${tokens.border}`,
                borderRadius: 6,
                color: tokens.muted,
                fontSize: 13,
                fontWeight: 500,
                cursor: 'pointer',
                transition: 'border-color 200ms, color 200ms',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = tokens.muted;
                e.currentTarget.style.color = tokens.text;
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = tokens.border;
                e.currentTarget.style.color = tokens.muted;
              }}
            >
              Skip
            </button>

            {currentStep < 3 ? (
              <button
                onClick={handleNext}
                disabled={!canProceedFromStep1 && currentStep === 1}
                style={{
                  padding: '10px 24px',
                  background: canProceedFromStep1 || currentStep > 1 ? tokens.cta : `${tokens.cta}40`,
                  border: 'none',
                  borderRadius: 6,
                  color: tokens.bg,
                  fontSize: 13,
                  fontWeight: 600,
                  cursor: (canProceedFromStep1 || currentStep > 1) ? 'pointer' : 'not-allowed',
                  opacity: (canProceedFromStep1 || currentStep > 1) ? 1 : 0.5,
                }}
              >
                Next &rarr;
              </button>
            ) : (
              <button
                onClick={handleFinish}
                style={{
                  padding: '10px 24px',
                  background: tokens.cta,
                  border: 'none',
                  borderRadius: 6,
                  color: tokens.bg,
                  fontSize: 13,
                  fontWeight: 600,
                  cursor: 'pointer',
                }}
              >
                Start Using OpenForge
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
