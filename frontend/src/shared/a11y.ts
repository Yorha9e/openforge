import {
  createElement,
  type ReactNode,
  type ReactElement,
  type KeyboardEvent,
  type CSSProperties,
  type AnchorHTMLAttributes,
  type HTMLAttributes,
} from 'react';

// ---------------------------------------------------------------------------
// Selectors
// ---------------------------------------------------------------------------

/** CSS selector matching all keyboard-focusable elements. */
export const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'textarea:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

// ---------------------------------------------------------------------------
// Focus trap
// ---------------------------------------------------------------------------

/**
 * Traps Tab focus within `container`.
 * Returns a cleanup function that restores focus to the previously-active
 * element.  Call it when the modal / dialog closes.
 */
export function trapFocus(container: HTMLElement): () => void {
  const previouslyFocused = document.activeElement as HTMLElement | null;

  const handleKeyDown = (e: globalThis.KeyboardEvent): void => {
    if (e.key !== 'Tab') return;

    const focusable = container.querySelectorAll<HTMLElement>(focusableSelector);
    if (focusable.length === 0) return;

    const first = focusable[0] as HTMLElement;
    const last = focusable[focusable.length - 1] as HTMLElement;

    if (e.shiftKey) {
      if (document.activeElement === first) {
        e.preventDefault();
        last.focus();
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  };

  document.addEventListener('keydown', handleKeyDown);

  // Move focus into the container
  container.querySelector<HTMLElement>(focusableSelector)?.focus();

  return () => {
    document.removeEventListener('keydown', handleKeyDown);
    previouslyFocused?.focus();
  };
}

// ---------------------------------------------------------------------------
// Screen-reader announcements
// ---------------------------------------------------------------------------

let announcer: HTMLDivElement | null = null;

/**
 * Pushes a message to an off-screen aria-live region so screen readers
 * announce it without visual disruption.
 */
export function announce(
  message: string,
  priority: 'polite' | 'assertive' = 'polite',
): void {
  if (!announcer) {
    announcer = document.createElement('div');
    announcer.setAttribute('aria-live', priority);
    announcer.setAttribute('aria-atomic', 'true');
    announcer.style.cssText =
      'position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0;';
    document.body.appendChild(announcer);
  }

  announcer.setAttribute('aria-live', priority);

  // Clear then re-set so the mutation observer fires even for identical text
  announcer.textContent = '';
  requestAnimationFrame(() => {
    if (announcer) announcer.textContent = message;
  });
}

// ---------------------------------------------------------------------------
// Skip-link component
// ---------------------------------------------------------------------------

interface SkipLinkProps {
  /** ID of the main-content element (without `#`). */
  href?: string;
  /** Visible label. */
  children?: ReactNode;
}

/**
 * Renders a skip-to-content link that becomes visible on keyboard focus.
 * Place at the very top of the page, before the primary navigation.
 */
export function SkipLink({
  href = '#main-content',
  children = 'Skip to content',
}: SkipLinkProps): ReactElement {
  const linkProps: AnchorHTMLAttributes<HTMLAnchorElement> & {
    onFocus: (e: React.FocusEvent<HTMLAnchorElement>) => void;
    onBlur: (e: React.FocusEvent<HTMLAnchorElement>) => void;
  } = {
    href,
    style: {
      position: 'absolute',
      top: -9999,
      left: -9999,
      zIndex: 9999,
      background: 'var(--of-bg-primary, #fff)',
      color: 'var(--of-text-primary, #000)',
      padding: '8px 16px',
    },
    onFocus(e) {
      const el = e.currentTarget;
      el.style.position = 'fixed';
      el.style.top = '8px';
      el.style.left = '8px';
      el.style.outline = '2px solid var(--of-accent, #06c)';
      el.style.outlineOffset = '2px';
    },
    onBlur(e) {
      const el = e.currentTarget;
      el.style.position = 'absolute';
      el.style.top = '-9999px';
      el.style.left = '-9999px';
      el.style.outline = '';
      el.style.outlineOffset = '';
    },
  };

  return createElement('a', linkProps, children);
}

// ---------------------------------------------------------------------------
// Convenience helpers
// ---------------------------------------------------------------------------

export const a11y = {
  /** ARIA attributes for a labelled panel / region. */
  panel(label: string): HTMLAttributes<HTMLElement> {
    return { role: 'region', 'aria-label': label };
  },

  /** ARIA attributes for a live region (streaming updates). */
  live(priority: 'polite' | 'assertive' = 'polite'): HTMLAttributes<HTMLElement> {
    return { 'aria-live': priority };
  },

  /** ARIA attributes for a keyboard-navigable interactive node. */
  node(label: string): HTMLAttributes<HTMLElement> & { tabIndex: number } {
    return { role: 'button', 'aria-label': label, tabIndex: 0 };
  },

  /** Keyboard-event handler for Enter / Space activation. */
  onActivate(handler: () => void): (e: KeyboardEvent) => void {
    return (e: KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        handler();
      }
    };
  },

  /** Focus-ring style that respects the `--of-accent` theme token. */
  focusRing: {
    outline: '2px solid var(--of-accent)',
    outlineOffset: '2px',
  } satisfies CSSProperties,

  /** Anchor props for a skip-link element. */
  skipLink(id: string): AnchorHTMLAttributes<HTMLAnchorElement> {
    return {
      href: `#${id}`,
      style: {
        position: 'absolute',
        top: -9999,
        left: -9999,
      },
    };
  },
};
