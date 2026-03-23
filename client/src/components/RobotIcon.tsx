import React from 'react';

export default function RobotIcon() {
  return (
    <svg width="140" height="140" viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg" className="robot-avatar">
      {/* 底部光晕 */}
      <ellipse cx="50" cy="90" rx="30" ry="6" fill="#8b5cf6" fillOpacity="0.3" filter="blur(8px)">
        <animate attributeName="rx" values="30;25;30" dur="3s" repeatCount="indefinite" />
        <animate attributeName="fill-opacity" values="0.3;0.1;0.3" dur="3s" repeatCount="indefinite" />
      </ellipse>

      <g className="robot-float-group">
        {/* 天线 */}
        <line x1="50" y1="22" x2="50" y2="10" stroke="#a78bfa" strokeWidth="2" strokeLinecap="round" />
        <circle cx="50" cy="8" r="3.5" fill="#ec4899" className="robot-antenna-ball" />

        {/* 耳朵/耳机 */}
        <rect x="18" y="38" width="6" height="16" rx="2" fill="#4c1d95" />
        <rect x="76" y="38" width="6" height="16" rx="2" fill="#4c1d95" />

        {/* 脸部轮廓 */}
        <rect x="22" y="22" width="56" height="48" rx="16" fill="url(#face-gradient)" stroke="rgba(255,255,255,0.15)" strokeWidth="1.5" />
        
        {/* 屏幕/面罩区域 */}
        <rect x="28" y="32" width="44" height="28" rx="10" fill="#0f172a" fillOpacity="0.9" stroke="rgba(255,255,255,0.05)" strokeWidth="1" />

        {/* 眼睛 (带眨眼动画) */}
        <g className="robot-eyes">
          <ellipse cx="40" cy="44" rx="5" ry="7" fill="#38bdf8" className="eye-blink" />
          <ellipse cx="60" cy="44" rx="5" ry="7" fill="#38bdf8" className="eye-blink" />
          
          {/* 眼睛高光 */}
          <circle cx="42" cy="41" r="1.5" fill="white" fillOpacity="0.8" className="eye-blink" />
          <circle cx="62" cy="41" r="1.5" fill="white" fillOpacity="0.8" className="eye-blink" />
        </g>
        
        {/* 嘴巴 (微笑) */}
        <path d="M43 53 Q50 56 57 53" stroke="#38bdf8" strokeWidth="2" strokeLinecap="round" strokeOpacity="0.5" />

        {/* 身体简略 */}
        <path d="M32 75 Q50 88 68 75 L68 92 Q50 96 32 92 Z" fill="url(#body-gradient)" opacity="0.9" />
        {/* 领口细节 */}
        <path d="M38 75 Q50 82 62 75" stroke="rgba(255,255,255,0.1)" strokeWidth="1" fill="none" />
      </g>

      {/* 渐变定义 */}
      <defs>
        <linearGradient id="face-gradient" x1="22" y1="22" x2="78" y2="70" gradientUnits="userSpaceOnUse">
          <stop stopColor="#5b21b6" />
          <stop offset="1" stopColor="#be185d" />
        </linearGradient>
        <linearGradient id="body-gradient" x1="32" y1="75" x2="68" y2="92" gradientUnits="userSpaceOnUse">
          <stop stopColor="#6d28d9" />
          <stop offset="1" stopColor="#db2777" />
        </linearGradient>
      </defs>
    </svg>
  );
}
