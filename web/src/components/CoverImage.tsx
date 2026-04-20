import { useState } from 'react';
import { encodeOurn } from '../api/fetcher';

interface Props {
  ourn: string;
  title: string;
}

// Deterministic hue from ourn so each missing cover gets a consistent colour
function hashToHsl(str: string): string {
  let h = 0;
  for (let i = 0; i < str.length; i++) {
    h = (Math.imul(31, h) + str.charCodeAt(i)) | 0;
  }
  return `hsl(${Math.abs(h) % 360}, 40%, 30%)`;
}

function Placeholder({ ourn, title }: Props) {
  const letter = title.charAt(0).toUpperCase() || '?';
  return (
    <svg
      viewBox="0 0 2 3"
      xmlns="http://www.w3.org/2000/svg"
      className="h-full w-full drop-shadow-md"
      aria-hidden="true"
    >
      <rect width="2" height="3" fill={hashToHsl(ourn)} />
      <text
        x="1"
        y="1.75"
        textAnchor="middle"
        fontSize="1.1"
        fontWeight="bold"
        fill="white"
        fontFamily="system-ui, sans-serif"
      >
        {letter}
      </text>
    </svg>
  );
}

export function CoverImage({ ourn, title }: Props) {
  const [failed, setFailed] = useState(false);

  if (failed) return <Placeholder ourn={ourn} title={title} />;

  return (
    <img
      src={`/api/covers/${encodeOurn(ourn)}/400w`}
      alt={title}
      loading="lazy"
      decoding="async"
      onError={() => setFailed(true)}
      className="h-full w-full object-contain drop-shadow-md"
    />
  );
}
