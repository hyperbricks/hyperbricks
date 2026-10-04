import { Ball, BonusPopup, Drone, Particle } from './entities';
import { SoundEngine } from './sound-engine';
import { COURT, CPU_DIFFICULTY, type Direction, type Mode, type Phase, type Player } from './types';

export interface GameElements {
  canvas: HTMLCanvasElement; play: HTMLButtonElement; pause: HTMLButtonElement; restart: HTMLButtonElement; sound: HTMLButtonElement; difficulty: HTMLSelectElement;
  overlay: HTMLElement; overlayTitle: HTMLElement; overlayCopy: HTMLElement; overlayHint: HTMLElement; status: HTMLElement; leftScore: HTMLElement; rightScore: HTMLElement;
  rally: HTMLElement; announcement: HTMLElement; modeLabel: HTMLElement; leftLabel: HTMLElement; rightLabel: HTMLElement; modeButtons: HTMLButtonElement[];
}

export class PongGame {
  private readonly context: CanvasRenderingContext2D;
  private readonly sound = new SoundEngine();
  private readonly ball = new Ball();
  private readonly drones = [new Drone(0), new Drone(1), new Drone(2)];
  private readonly keys = new Set<string>();
  private readonly pointers = new Map<number, Player>();
  private readonly reducedMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;
  private phase: Phase = 'ready'; private mode: Mode = 'solo'; private scores: [number, number] = [0, 0]; private paddles: [number, number] = [310, 310];
  private lastHitter: Player | null = null; private trail: { x: number; y: number }[] = []; private particles: Particle[] = []; private popups: BonusPopup[] = [];
  private lastFrame = 0; private countdown = 0; private rallyCount = 0; private elapsed = 0; private shake = 0; private aiTarget = 310; private aiTimer = 0;

  constructor(private readonly elements: GameElements) {
    const context = elements.canvas.getContext('2d'); if (!context) throw new Error('Canvas 2D rendering is unavailable.'); this.context = context;
    this.bindControls(); this.reset(); requestAnimationFrame(this.frame);
  }

  private readonly frame = (time: number): void => { const delta = Math.min((time - this.lastFrame) / 1000 || 0, .04); this.lastFrame = time; if (this.phase === 'playing') { const steps = Math.max(1, Math.ceil(delta / .008)); for (let step = 0; step < steps && this.phase === 'playing'; step++) this.update(delta / steps); } this.render(delta); requestAnimationFrame(this.frame); };
  private bindControls(): void {
    this.elements.play.addEventListener('click', this.start); this.elements.pause.addEventListener('click', this.togglePause); this.elements.restart.addEventListener('click', this.reset); this.elements.sound.addEventListener('click', this.toggleSound);
    this.elements.modeButtons.forEach(button => button.addEventListener('click', () => this.setMode(button.dataset.mode === 'duo' ? 'duo' : 'solo')));
    window.addEventListener('keydown', this.onKeyDown); window.addEventListener('keyup', this.onKeyUp); window.addEventListener('blur', this.onBlur); document.addEventListener('visibilitychange', this.onVisibilityChange);
    this.elements.canvas.addEventListener('pointerdown', this.onPointerDown); this.elements.canvas.addEventListener('pointermove', this.onPointerMove);
    this.elements.canvas.addEventListener('pointerup', this.onPointerEnd); this.elements.canvas.addEventListener('pointercancel', this.onPointerEnd); this.elements.canvas.addEventListener('lostpointercapture', this.onPointerEnd);
  }
  private readonly onKeyDown = (event: KeyboardEvent): void => { if (event.target instanceof Element && event.target.closest('button,select,input,a')) return; const key = event.key.toLowerCase(); if (['w', 's', 'arrowup', 'arrowdown', ' '].includes(key)) event.preventDefault(); if (key === ' ' && !event.repeat) this.phase === 'ready' || this.phase === 'over' ? this.start() : this.togglePause(); if (key === 'escape' && !event.repeat && (this.phase === 'playing' || this.phase === 'paused')) this.togglePause(); this.keys.add(key); };
  private readonly onKeyUp = (event: KeyboardEvent): void => { this.keys.delete(event.key.toLowerCase()); };
  private readonly onBlur = (): void => { this.keys.clear(); if (this.phase === 'playing') this.pause(); };
  private readonly onVisibilityChange = (): void => { if (document.hidden && this.phase === 'playing') this.pause(); };
  private readonly onPointerDown = (event: PointerEvent): void => { if (this.phase !== 'playing') return; const rect = this.elements.canvas.getBoundingClientRect(); const player: Player = this.mode === 'duo' && event.clientX - rect.left > rect.width / 2 ? 1 : 0; this.pointers.set(event.pointerId, player); this.elements.canvas.setPointerCapture(event.pointerId); this.movePointer(event); };
  private readonly onPointerMove = (event: PointerEvent): void => this.movePointer(event);
  private readonly onPointerEnd = (event: PointerEvent): void => { this.pointers.delete(event.pointerId); };
  private movePointer(event: PointerEvent): void { if (this.phase !== 'playing') return; const player = this.pointers.get(event.pointerId); if (player === undefined) return; const rect = this.elements.canvas.getBoundingClientRect(); this.paddles[player] = this.clampPaddle((event.clientY - rect.top) / rect.height * COURT.height); }

  private readonly start = (): void => { this.sound.unlock(); if (this.phase === 'paused') this.phase = 'playing'; else { this.scores = [0, 0]; this.paddles = [310, 310]; this.particles = []; this.resetDrones(); this.phase = 'playing'; this.serve(); this.sound.tone(220, .15, 'triangle', 660); } this.hideOverlay(); this.updateHud(); this.elements.canvas.focus({ preventScroll: true }); };
  private readonly togglePause = (): void => { if (this.phase === 'playing') this.pause(); else if (this.phase === 'paused') this.start(); };
  private readonly reset = (): void => { this.elements.announcement.textContent = ''; this.phase = 'ready'; this.scores = [0, 0]; this.paddles = [310, 310]; this.keys.clear(); this.pointers.clear(); this.particles = []; this.resetDrones(); this.serve(); this.showOverlay('READY TO RALLY?', this.mode === 'solo' ? 'Outplay the machine. Own the court.' : 'Grab a rival. Settle it on the court.', 'LET’S PLAY'); this.updateHud(); };
  private readonly toggleSound = (): void => { this.sound.enabled = !this.sound.enabled; if (this.sound.enabled) { this.sound.unlock(); this.sound.tone(660); } this.elements.sound.textContent = this.sound.enabled ? '♪ SOUND ON' : '♪ SOUND OFF'; this.elements.sound.setAttribute('aria-pressed', String(this.sound.enabled)); };
  private pause(): void { this.phase = 'paused'; this.keys.clear(); this.pointers.clear(); this.showOverlay('TAKE A BREATHER.', 'Your match is right here when you’re ready.', 'RESUME MATCH'); this.updateHud(); }
  private setMode(mode: Mode): void { this.mode = mode; this.elements.modeButtons.forEach(button => { const active = button.dataset.mode === mode; button.classList.toggle('selected', active); button.setAttribute('aria-pressed', String(active)); }); this.elements.modeLabel.textContent = mode === 'solo' ? 'SOLO MATCH' : 'VERSUS MATCH'; this.elements.leftLabel.textContent = mode === 'solo' ? 'YOU' : 'PLAYER 1'; this.elements.rightLabel.textContent = mode === 'solo' ? 'CPU' : 'PLAYER 2'; this.elements.difficulty.disabled = mode === 'duo'; this.elements.overlayHint.textContent = mode === 'solo' ? 'W / S OR ↑ / ↓ TO MOVE' : 'PLAYER 1: W / S · PLAYER 2: ↑ / ↓'; this.reset(); }
  private serve(direction: Direction = Math.random() < .5 ? -1 : 1): void { this.ball.serve(direction); this.lastHitter = null; this.trail = []; this.rallyCount = 0; this.countdown = 1.15; }
  private resetDrones(): void { this.drones.forEach(drone => drone.respawn()); this.popups = []; this.elapsed = 0; }
  private clampPaddle(value: number): number { return Math.max(COURT.paddleHeight / 2 + 12, Math.min(COURT.height - COURT.paddleHeight / 2 - 12, value)); }

  private update(delta: number): void {
    this.elapsed += delta; this.popups = this.popups.filter(popup => popup.update(delta)); this.particles = this.particles.filter(particle => particle.update(delta)); this.drones.forEach(drone => drone.update(delta, this.elapsed, this.reducedMotion)); this.movePaddles(delta);
    if (this.countdown > 0) { this.countdown -= delta; if (this.countdown <= 0) { this.sound.serve(); this.updateHud(); } return; }
    this.ball.move(delta); this.collideWalls(); this.collidePaddle(); this.collideDrones();
    if (this.ball.x < -20) this.score(1); else if (this.ball.x > COURT.width + 20) this.score(0);
  }
  private movePaddles(delta: number): void {
    const up = this.keys.has('w') || (this.mode === 'solo' && this.keys.has('arrowup')), down = this.keys.has('s') || (this.mode === 'solo' && this.keys.has('arrowdown'));
    this.paddles[0] = this.clampPaddle(this.paddles[0] + ((down ? 1 : 0) - (up ? 1 : 0)) * 650 * delta);
    if (this.mode === 'duo') { this.paddles[1] = this.clampPaddle(this.paddles[1] + ((this.keys.has('arrowdown') ? 1 : 0) - (this.keys.has('arrowup') ? 1 : 0)) * 650 * delta); return; }
    const setting = CPU_DIFFICULTY[this.elements.difficulty.value] ?? { speed: 400, reaction: .16, aimVariance: 55 };
    this.aiTimer -= delta; if (this.aiTimer <= 0) { this.aiTimer = setting.reaction; this.aiTarget = (this.ball.vx > 0 ? this.ball.y : COURT.height / 2) + (Math.random() - .5) * setting.aimVariance; }
    this.paddles[1] = this.clampPaddle(this.paddles[1] + Math.max(-setting.speed * delta, Math.min(setting.speed * delta, this.aiTarget - this.paddles[1])));
  }
  private collideWalls(): void { if (this.ball.y >= 10 && this.ball.y <= COURT.height - 10) return; this.ball.y = Math.max(10, Math.min(COURT.height - 10, this.ball.y)); this.ball.vy *= -1; this.sound.wall(); this.burst(this.ball.x, this.ball.y, '#a3abc8'); }
  private collidePaddle(): void {
    const player: Player = this.ball.vx < 0 ? 0 : 1, face = player === 0 ? COURT.paddleInset + COURT.paddleWidth : COURT.width - COURT.paddleInset - COURT.paddleWidth;
    const atFace = player === 0 ? this.ball.x - 9 <= face && this.ball.x > 35 : this.ball.x + 9 >= face && this.ball.x < COURT.width - 35;
    if (!atFace || Math.abs(this.ball.y - this.paddles[player]) >= COURT.paddleHeight / 2 + 9) return;
    this.ball.rebound(player, this.paddles[player]); this.lastHitter = player; this.rallyCount++; this.sound.paddle(player, this.rallyCount); this.burst(this.ball.x, this.ball.y, player === 0 ? '#64f8e4' : '#ff689e'); this.updateHud();
  }
  private collideDrones(): void {
    if (this.lastHitter === null) return;
    for (const drone of this.drones) { if (!drone.hitBy(this.ball)) continue; drone.destroy(); this.burst(drone.x, drone.y, '#ffd66b'); this.popups.push(new BonusPopup(drone.x, drone.y, this.lastHitter)); this.score(this.lastHitter, true); if (this.phase === 'over') return; }
  }
  private score(player: Player, bonus = false): void {
    this.scores[player]++; this.burst(this.ball.x, this.ball.y, player === 0 ? '#64f8e4' : '#ff689e');
    this.elements.announcement.textContent = `${bonus ? 'Drone destroyed! Bonus point. ' : ''}${this.mode === 'solo' ? 'You' : 'Player one'} ${this.scores[0]}, ${this.mode === 'solo' ? 'CPU' : 'player two'} ${this.scores[1]}.`;
    if (this.scores[player] >= 7) { this.phase = 'over'; this.sound.match(player === 0 || this.mode === 'duo'); this.showOverlay(this.mode === 'solo' ? player === 0 ? 'YOU OWN THE COURT.' : 'THE MACHINE WINS.' : `PLAYER ${player + 1} WINS!`, `${this.scores[0]} — ${this.scores[1]} · Another round?`, 'PLAY AGAIN'); }
    else if (bonus) this.sound.drone(); else { this.sound.score(); this.serve(player === 0 ? 1 : -1); }
    this.updateHud();
  }
  private burst(x: number, y: number, color: string): void { if (this.reducedMotion) return; for (let index = 0; index < 15; index++) this.particles.push(new Particle(x, y, (Math.random() - .5) * 360, (Math.random() - .5) * 360, color)); this.shake = 5; }
  private render(delta: number): void {
    const context = this.context; context.clearRect(0, 0, COURT.width, COURT.height); context.save();
    if (!this.reducedMotion && this.shake > .1) { context.translate((Math.random() - .5) * this.shake, (Math.random() - .5) * this.shake); this.shake *= Math.pow(.002, delta); }
    this.renderCourt(); this.drones.forEach(drone => drone.render(context)); this.popups.forEach(popup => popup.render(context, this.reducedMotion)); this.renderPaddles(); this.renderBall(); this.particles.forEach(particle => particle.render(context)); context.globalAlpha = 1;
    if (this.phase === 'playing' && this.countdown > 0) { context.fillStyle = '#edf0fa'; context.font = 'bold 52px monospace'; context.textAlign = 'center'; context.fillText('GET READY', COURT.width / 2, COURT.height / 2 - 100); } context.restore();
  }
  private renderCourt(): void { const context = this.context; context.strokeStyle = '#192132'; context.lineWidth = 1; for (let x = 0; x <= COURT.width; x += 60) { context.beginPath(); context.moveTo(x, 0); context.lineTo(x, COURT.height); context.stroke(); } for (let y = 0; y <= COURT.height; y += 60) { context.beginPath(); context.moveTo(0, y); context.lineTo(COURT.width, y); context.stroke(); } context.setLineDash([9, 14]); context.strokeStyle = '#3a445a'; context.beginPath(); context.moveTo(COURT.width / 2, 20); context.lineTo(COURT.width / 2, COURT.height - 20); context.stroke(); context.setLineDash([]); context.beginPath(); context.arc(COURT.width / 2, COURT.height / 2, 76, 0, Math.PI * 2); context.strokeStyle = '#273147'; context.stroke(); }
  private renderPaddles(): void { this.paddles.forEach((center, player) => { const color = player === 0 ? '#64f8e4' : '#ff689e'; this.context.fillStyle = color; this.context.shadowColor = color; this.context.shadowBlur = this.reducedMotion ? 0 : 22; this.context.fillRect(player === 0 ? COURT.paddleInset : COURT.width - COURT.paddleInset - COURT.paddleWidth, center - COURT.paddleHeight / 2, COURT.paddleWidth, COURT.paddleHeight); this.context.shadowBlur = 0; }); }
  private renderBall(): void { if (!this.reducedMotion && this.phase === 'playing' && this.countdown <= 0) { this.trail.push({ x: this.ball.x, y: this.ball.y }); if (this.trail.length > 18) this.trail.shift(); } this.trail.forEach((point, index) => { this.context.fillStyle = `rgba(190,230,255,${index / this.trail.length * .25})`; this.context.beginPath(); this.context.arc(point.x, point.y, 3 + index / this.trail.length * 5, 0, Math.PI * 2); this.context.fill(); }); this.ball.render(this.context, this.reducedMotion); }
  private updateHud(): void { this.elements.leftScore.textContent = String(this.scores[0]).padStart(2, '0'); this.elements.rightScore.textContent = String(this.scores[1]).padStart(2, '0'); this.elements.rally.textContent = String(this.rallyCount).padStart(2, '0'); this.elements.status.textContent = this.phase === 'playing' ? this.countdown > 0 ? 'NEXT SERVE' : this.rallyCount >= 8 ? 'HEATING UP' : 'MATCH IN PROGRESS' : this.phase === 'paused' ? 'MATCH PAUSED' : this.phase === 'over' ? 'MATCH COMPLETE' : 'AWAITING CHALLENGER'; this.elements.pause.disabled = this.phase !== 'playing' && this.phase !== 'paused'; this.elements.pause.textContent = this.phase === 'paused' ? '▶ RESUME' : 'Ⅱ PAUSE'; }
  private showOverlay(title: string, copy: string, button: string): void { this.elements.overlay.classList.remove('hidden'); this.elements.overlayTitle.textContent = title; this.elements.overlayCopy.textContent = copy; this.elements.play.textContent = `${button} ↗`; }
  private hideOverlay(): void { this.elements.overlay.classList.add('hidden'); }
}
