export type Player = 0 | 1;
export type Mode = 'solo' | 'duo';
export type Phase = 'ready' | 'playing' | 'paused' | 'over';
export type Direction = -1 | 1;

export interface Difficulty { speed: number; reaction: number; aimVariance: number; }

export const COURT = { width: 1200, height: 620, paddleHeight: 112, paddleWidth: 13, paddleInset: 51 } as const;
export const CPU_DIFFICULTY: Readonly<Record<string, Difficulty>> = {
  easy: { speed: 280, reaction: .25, aimVariance: 95 },
  normal: { speed: 400, reaction: .16, aimVariance: 55 },
  hard: { speed: 530, reaction: .09, aimVariance: 22 }
};
