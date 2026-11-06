interface MarkProps { size?: number; light?: boolean }
export default function BrandMark({ size = 42, light = true }: MarkProps) {
  return <svg width={size} height={size} viewBox="0 0 64 64" role="img" aria-label="Firedoor geometric doorway mark">
    <path d="M12 53V14L32 6L52 14V53" fill="none" stroke={light ? '#F2EDE3' : '#243B45'} strokeWidth="5" strokeLinejoin="round" />
    <path d="M31 16L47 22V45L31 39Z" fill="#C84938" />
    <path d="M4 32H25" fill="none" stroke="#E2AC53" strokeWidth="5" />
  </svg>;
}
