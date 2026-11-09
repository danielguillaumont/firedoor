// A bounded initial-screen deep link for local screenshot reproducibility.
// This selects a view, never a network API path, approval mode, or evidence file.
// In particular, opening the planning screen cannot unlock historical records.
export function initialDemoViewFromSearch(search: string): 'overview' | 'containment' {
  const params = new URLSearchParams(search);
  const entries = [...params.entries()];
  return entries.length === 1 && entries[0][0] === 'view' && entries[0][1] === 'containment'
    ? 'containment' : 'overview';
}
