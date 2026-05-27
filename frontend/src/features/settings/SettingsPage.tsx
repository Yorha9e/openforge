import { useEffect, useState, useCallback } from 'react';
import { api } from '../../shared/api';
import { AppLayout } from '../../shared/AppLayout';
import { useTheme, type Theme } from '../../shared/theme-provider';
import { tokens } from '../../shared/design-tokens';

interface Settings {
  notifications: { emailEnabled: boolean; webhookUrl: string; channels: string[] };
  layout: { editorFontSize: number; theme: Theme; defaultViewMode: 'simple' | 'pro' };
  language: { locale: string; timezone: string };
  project: { workDir: string };
}

const DEFAULT_SETTINGS: Settings = {
  notifications: { emailEnabled: true, webhookUrl: '', channels: ['email'] },
  layout: { editorFontSize: 14, theme: 'dark', defaultViewMode: 'pro' },
  language: { locale: 'en', timezone: 'UTC' },
  project: { workDir: '' },
};

const CHANNEL_OPTIONS = ['email', 'slack', 'webhook', 'sms'];
const LOCALE_OPTIONS = [
  { value: 'en', label: 'English' }, { value: 'zh', label: '中文' },
  { value: 'ja', label: '日本語' }, { value: 'ko', label: '한국어' },
  { value: 'de', label: 'Deutsch' }, { value: 'fr', label: 'Français' },
];
const TIMEZONE_OPTIONS = [
  'UTC', 'America/New_York', 'America/Chicago', 'America/Denver',
  'America/Los_Angeles', 'Europe/London', 'Europe/Berlin', 'Europe/Paris',
  'Asia/Shanghai', 'Asia/Tokyo', 'Asia/Kolkata', 'Australia/Sydney', 'Pacific/Auckland',
];

function Toggle({ enabled, onChange, label }: { enabled: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
      <div onClick={() => onChange(!enabled)}
        style={{
          width: 40, height: 22, borderRadius: 11, background: enabled ? tokens.cta : tokens.border,
          position: 'relative', transition: 'background 200ms', cursor: 'pointer', flexShrink: 0,
        }}>
        <div style={{
          width: 18, height: 18, borderRadius: '50%', background: tokens.text,
          position: 'absolute', top: 2, left: enabled ? 20 : 2, transition: 'left 200ms',
        }} />
      </div>
      <span style={{ fontSize: 13, color: tokens.text }}>{label}</span>
    </label>
  );
}

function SettingsCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ background: tokens.surface, borderRadius: 8, padding: 20, border: `1px solid ${tokens.border}` }}>
      <h2 style={{ fontFamily: tokens.fontHeading, fontSize: 16, fontWeight: 600, margin: '0 0 16px 0', color: tokens.text }}>{title}</h2>
      {children}
    </div>
  );
}

export default function SettingsPage() {
  const { theme, setTheme } = useTheme();
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  useEffect(() => {
    api.getSettings()
      .then((data: any) => {
        if (data) {
          setSettings({
            notifications: {
              emailEnabled: data.notifications?.emailEnabled ?? DEFAULT_SETTINGS.notifications.emailEnabled,
              webhookUrl: data.notifications?.webhookUrl ?? DEFAULT_SETTINGS.notifications.webhookUrl,
              channels: data.notifications?.channels ?? DEFAULT_SETTINGS.notifications.channels,
            },
            layout: {
              editorFontSize: data.layout?.editorFontSize ?? DEFAULT_SETTINGS.layout.editorFontSize,
              theme: data.layout?.theme ?? DEFAULT_SETTINGS.layout.theme,
              defaultViewMode: data.layout?.defaultViewMode ?? DEFAULT_SETTINGS.layout.defaultViewMode,
            },
            language: {
              locale: data.language?.locale ?? DEFAULT_SETTINGS.language.locale,
              timezone: data.language?.timezone ?? DEFAULT_SETTINGS.language.timezone,
            },
            project: {
              workDir: data.project?.workDir ?? localStorage.getItem('openforge_work_dir') ?? '',
            },
          });
        }
      })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  const updateNested = useCallback(
    <K1 extends keyof Settings, K2 extends keyof Settings[K1]>(section: K1, field: K2, value: Settings[K1][K2]) => {
      setSettings(prev => ({ ...prev, [section]: { ...prev[section], [field]: value } }));
    }, []);

  const handleSave = useCallback(async () => {
    setSaving(true); setFeedback(null);
    try {
      await api.updateSettings(settings);
      setFeedback({ type: 'success', message: 'Settings saved successfully.' });
    } catch (err: any) {
      setFeedback({ type: 'error', message: err.message || 'Failed to save settings.' });
    } finally { setSaving(false); }
  }, [settings]);

  const toggleChannel = useCallback((ch: string) => {
    setSettings(prev => {
      const channels = prev.notifications.channels.includes(ch)
        ? prev.notifications.channels.filter(c => c !== ch)
        : [...prev.notifications.channels, ch];
      return { ...prev, notifications: { ...prev.notifications, channels } };
    });
  }, []);

  if (loading) {
    return <AppLayout><p style={{ color: tokens.muted }}>Loading settings...</p></AppLayout>;
  }

  return (
    <AppLayout>
      <div style={{ maxWidth: 640, display: 'flex', flexDirection: 'column', gap: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, fontFamily: tokens.fontHeading, margin: 0, color: tokens.text }}>Settings</h1>

        {feedback && (
          <div style={{
            padding: '10px 16px', borderRadius: 6, fontSize: 13,
            background: feedback.type === 'success' ? '#064E3B' : '#7F1D1D',
            border: feedback.type === 'success' ? '1px solid #22C55E' : '1px solid #EF4444',
            color: feedback.type === 'success' ? '#BBF7D0' : '#FECACA',
          }}>
            {feedback.message}
          </div>
        )}

        <SettingsCard title="Notifications">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <Toggle enabled={settings.notifications.emailEnabled} onChange={v => updateNested('notifications', 'emailEnabled', v)} label="Email notifications" />
            <div>
              <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Webhook URL</label>
              <input type="url" value={settings.notifications.webhookUrl}
                onChange={e => updateNested('notifications', 'webhookUrl', e.target.value)}
                placeholder="https://hooks.example.com/notify"
                style={{
                  width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`,
                  borderRadius: 4, color: tokens.text, fontSize: 13, outline: 'none', boxSizing: 'border-box',
                }} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Notification Channels</label>
              <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
                {CHANNEL_OPTIONS.map(ch => (
                  <label key={ch} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13, cursor: 'pointer' }}>
                    <input type="checkbox" checked={settings.notifications.channels.includes(ch)} onChange={() => toggleChannel(ch)} style={{ accentColor: tokens.cta }} />
                    {ch.charAt(0).toUpperCase() + ch.slice(1)}
                  </label>
                ))}
              </div>
            </div>
          </div>
        </SettingsCard>

        <SettingsCard title="Layout">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Editor Font Size: {settings.layout.editorFontSize}px</label>
              <input type="range" min={12} max={24} step={1} value={settings.layout.editorFontSize}
                onChange={e => updateNested('layout', 'editorFontSize', Number(e.target.value))}
                style={{ width: '100%', accentColor: tokens.cta }} />
            </div>
            <div>
              <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Theme</label>
              <select value={theme} onChange={e => { const t = e.target.value as Theme; setTheme(t); updateNested('layout', 'theme', t); }}
                style={{
                  width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`,
                  borderRadius: 4, color: tokens.text, fontSize: 13, outline: 'none',
                }}>
                <option value="dark">Dark</option>
                <option value="light">Light</option>
                <option value="high-contrast">High Contrast</option>
              </select>
            </div>
            <div>
              <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Default View Mode</label>
              <select value={settings.layout.defaultViewMode}
                onChange={e => updateNested('layout', 'defaultViewMode', e.target.value as 'simple' | 'pro')}
                style={{
                  width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`,
                  borderRadius: 4, color: tokens.text, fontSize: 13, outline: 'none',
                }}>
                <option value="simple">Simple</option>
                <option value="pro">Pro</option>
              </select>
            </div>
          </div>
        </SettingsCard>

        <SettingsCard title="Language & Region">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            <div>
              <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Language</label>
              <select value={settings.language.locale} onChange={e => updateNested('language', 'locale', e.target.value)}
                style={{
                  width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`,
                  borderRadius: 4, color: tokens.text, fontSize: 13, outline: 'none',
                }}>
                {LOCALE_OPTIONS.map(opt => (<option key={opt.value} value={opt.value}>{opt.label}</option>))}
              </select>
            </div>
            <div>
              <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Timezone</label>
              <select value={settings.language.timezone} onChange={e => updateNested('language', 'timezone', e.target.value)}
                style={{
                  width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`,
                  borderRadius: 4, color: tokens.text, fontSize: 13, outline: 'none',
                }}>
                {TIMEZONE_OPTIONS.map(tz => (<option key={tz} value={tz}>{tz}</option>))}
              </select>
            </div>
          </div>
        </SettingsCard>

        <SettingsCard title="Project">
          <div>
            <label style={{ display: 'block', fontSize: 13, color: tokens.muted, marginBottom: 6 }}>Working Directory</label>
            <input type="text" value={settings.project.workDir}
              onChange={e => { updateNested('project', 'workDir', e.target.value); localStorage.setItem('openforge_work_dir', e.target.value); }}
              placeholder="/path/to/your/project (e.g., D:\projects\myapp)"
              style={{
                width: '100%', padding: '8px 12px', background: tokens.bg, border: `1px solid ${tokens.border}`,
                borderRadius: 4, color: tokens.text, fontSize: 13, outline: 'none', boxSizing: 'border-box',
              }} />
            <p style={{ fontSize: 12, color: '#64748b', marginTop: 6 }}>
              Absolute path to your project directory on the server. Tools (read_file, bash, etc.) will execute relative to this directory.
            </p>
          </div>
        </SettingsCard>

        <button onClick={handleSave} disabled={saving}
          style={{
            padding: '10px 24px', background: saving ? '#166534' : tokens.cta,
            color: tokens.ctaText, border: 'none', borderRadius: 6,
            fontSize: 14, fontWeight: 600, cursor: saving ? 'not-allowed' : 'pointer',
            alignSelf: 'flex-start', transition: tokens.transition,
          }}>
          {saving ? 'Saving...' : 'Save Settings'}
        </button>
      </div>
    </AppLayout>
  );
}
