import type { ReactNode } from 'react';
export type IconName = 'dashboard'|'layers'|'eye'|'shield'|'server'|'arrow'|'arrow-up'|'refresh'|'info'|'check'|'lock'|'chevron'|'arrow-left'|'activity'|'external'|'menu'|'alert'|'database'|'git'|'close';
const paths: Record<IconName, ReactNode> = {
  dashboard: <><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></>,
  layers: <><path d="m12 2 9 5-9 5-9-5 9-5Z"/><path d="m3 12 9 5 9-5M3 17l9 5 9-5"/></>,
  eye: <><path d="M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/></>,
  shield: <><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z"/></>,
  server: <><rect x="3" y="3" width="18" height="8" rx="2"/><rect x="3" y="13" width="18" height="8" rx="2"/><path d="M7 7h.01M7 17h.01"/></>,
  arrow: <><path d="M5 12h14m-6-6 6 6-6 6"/></>,
  'arrow-up': <><path d="M7 17 17 7M8 7h9v9"/></>,
  refresh: <><path d="M20 11a8 8 0 0 0-14-5L3 9m0-6v6h6M4 13a8 8 0 0 0 14 5l3-3m0 6v-6h-6"/></>,
  info: <><circle cx="12" cy="12" r="9"/><path d="M12 11v5m0-8h.01"/></>,
  check: <><path d="m5 12 4 4L19 6"/></>,
  lock: <><rect x="5" y="11" width="14" height="11" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></>,
  chevron: <><path d="m9 18 6-6-6-6"/></>,
  'arrow-left': <><path d="M19 12H5m6-6-6 6 6 6"/></>,
  activity: <><path d="M2 12h5l3-7 4 14 3-7h5"/></>,
  external: <><path d="M13 4h7v7M11 13l9-9M20 14v6H4V4h6"/></>,
  menu: <><path d="M4 6h16M4 12h16M4 18h16"/></>,
  alert: <><path d="m12 3 10 18H2L12 3Zm0 7v4m0 4h.01"/></>,
  database: <><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 4 18 4 18 0V5M3 12c0 4 18 4 18 0"/></>,
  close: <><path d="M18 6 6 18M6 6l12 12"/></>,
  git: <><circle cx="6" cy="5" r="2"/><circle cx="18" cy="18" r="2"/><circle cx="6" cy="18" r="2"/><path d="M6 7v9M8 5h4a6 6 0 0 1 6 6v5"/></>,
};
export default function Icon({ name, size = 18 }: { name: IconName; size?: number }) {
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.65" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}
