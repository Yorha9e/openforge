import { useState } from 'react';
import { tokens } from './design-tokens';

interface RoleSelectorProps {
  value: string;
  onChange: (role: string) => void;
  disabled?: boolean;
}

const roles = [
  { value: 'admin', label: 'Admin', description: 'Full system access' },
  { value: 'pm', label: 'Product Manager', description: 'Project management' },
  { value: 'dev_lead', label: 'Dev Lead', description: 'Technical leadership' },
  { value: 'dev', label: 'Developer', description: 'Development work' },
  { value: 'observer', label: 'Observer', description: 'Read-only access' },
];

export function RoleSelector({ value, onChange, disabled = false }: RoleSelectorProps) {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <div style={{ position: 'relative' }}>
      <label
        htmlFor="role-selector"
        style={{
          display: 'block',
          fontSize: 13,
          color: tokens.muted,
          marginBottom: 4,
          fontWeight: 500,
        }}
      >
        Role
      </label>
      <div
        id="role-selector"
        role="combobox"
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            if (!disabled) setIsOpen(!isOpen);
          }
          if (e.key === 'Escape') setIsOpen(false);
        }}
        tabIndex={disabled ? -1 : 0}
        style={{
          width: '100%',
          padding: '10px 12px',
          background: tokens.bg,
          border: `1px solid ${tokens.border}`,
          borderRadius: 4,
          color: tokens.text,
          cursor: disabled ? 'default' : 'pointer',
          opacity: disabled ? 0.5 : 1,
          boxSizing: 'border-box',
          fontSize: 14,
        }}
      >
        {value ? roles.find(r => r.value === value)?.label : 'Select a role'}
      </div>
      {isOpen && (
        <div
          role="listbox"
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            right: 0,
            background: tokens.surface,
            border: `1px solid ${tokens.border}`,
            borderRadius: 4,
            marginTop: 4,
            zIndex: 10,
            boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
          }}
        >
          {roles.map(role => (
            <div
              key={role.value}
              role="option"
              aria-selected={value === role.value}
              onClick={() => {
                onChange(role.value);
                setIsOpen(false);
              }}
              style={{
                padding: '10px 12px',
                cursor: 'pointer',
                borderBottom: `1px solid ${tokens.border}`,
                background: value === role.value ? tokens.cta : 'transparent',
                color: value === role.value ? tokens.ctaText : tokens.text,
              }}
            >
              <div style={{ fontWeight: 500 }}>{role.label}</div>
              <div style={{ fontSize: 12, color: tokens.muted }}>{role.description}</div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
