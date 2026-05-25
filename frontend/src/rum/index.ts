type RUMMetric = {
  name: string;
  value: number;
  rating: 'good' | 'needs-improvement' | 'poor';
  timestamp: number;
};

const buffer: RUMMetric[] = [];

function flush() {
  if (buffer.length === 0) return;
  const payload = buffer.splice(0);
  navigator.sendBeacon('/api/rum/metrics', JSON.stringify({ metrics: payload }));
}

// Flush every 30 seconds
setInterval(flush, 30_000);

export function trackRUM(metric: RUMMetric) {
  buffer.push(metric);
}

// Track uncaught errors
window.addEventListener('error', (event) => {
  navigator.sendBeacon('/api/rum/errors', JSON.stringify({
    message: event.message,
    filename: event.filename,
    lineno: event.lineno,
    colno: event.colno,
    timestamp: Date.now(),
  }));
});

window.addEventListener('unhandledrejection', (event) => {
  navigator.sendBeacon('/api/rum/errors', JSON.stringify({
    message: String(event.reason),
    type: 'unhandledrejection',
    timestamp: Date.now(),
  }));
});

export function initRUM() {
  if (!('PerformanceObserver' in window)) return;

  // LCP — largest-contentful-paint
  try {
    new PerformanceObserver((list) => {
      const entries = list.getEntries();
      const last = entries[entries.length - 1] as PerformanceEntry;
      trackRUM({
        name: 'LCP',
        value: last.startTime,
        rating: last.startTime < 2500 ? 'good' : last.startTime < 4000 ? 'needs-improvement' : 'poor',
        timestamp: Date.now(),
      });
    }).observe({ type: 'largest-contentful-paint', buffered: true });
  } catch { /* unsupported */ }

  // INP proxy via longtask
  try {
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        if (entry.duration > 200) {
          trackRUM({
            name: 'INP',
            value: entry.duration,
            rating: entry.duration < 200 ? 'good' : entry.duration < 500 ? 'needs-improvement' : 'poor',
            timestamp: Date.now(),
          });
        }
      }
    }).observe({ type: 'longtask', buffered: true });
  } catch { /* unsupported */ }

  // CLS — layout-shift
  try {
    let clsValue = 0;
    new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        const shift = entry as unknown as { hadRecentInput: boolean; value: number };
        if (!shift.hadRecentInput) {
          clsValue += shift.value;
        }
      }
      trackRUM({
        name: 'CLS',
        value: clsValue,
        rating: clsValue < 0.1 ? 'good' : clsValue < 0.25 ? 'needs-improvement' : 'poor',
        timestamp: Date.now(),
      });
    }).observe({ type: 'layout-shift', buffered: true });
  } catch { /* unsupported */ }
}
