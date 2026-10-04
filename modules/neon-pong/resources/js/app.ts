import { PongGame, type GameElements } from './game';

function required<T extends HTMLElement>(id: string): T {
  const element = document.getElementById(id);
  if (!element) throw new Error(`Missing required game element: #${id}`);
  return element as T;
}

const elements: GameElements = {
  canvas: required<HTMLCanvasElement>('game'), play: required<HTMLButtonElement>('play'), pause: required<HTMLButtonElement>('pause'), restart: required<HTMLButtonElement>('restart'), sound: required<HTMLButtonElement>('sound'), difficulty: required<HTMLSelectElement>('difficulty'),
  overlay: required('overlay'), overlayTitle: required('overlay-title'), overlayCopy: required('overlay-copy'), overlayHint: required('overlay-hint'), status: required('status'), leftScore: required('left-score'), rightScore: required('right-score'), rally: required('rally-count'), announcement: required('announcement'), modeLabel: required('mode-label'), leftLabel: required('left-label'), rightLabel: required('right-label'), modeButtons: Array.from(document.querySelectorAll<HTMLButtonElement>('[data-mode]'))
};

new PongGame(elements);
