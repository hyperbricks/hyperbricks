import { COURT, type Direction, type Player } from './types';

export class Ball {
  public x = COURT.width / 2;
  public y = COURT.height / 2;
  public vx = 0;
  public vy = 0;
  public speed = 440;

  serve(direction: Direction): void {
    const angle = (Math.random() - .5) * .8;
    this.x = COURT.width / 2; this.y = COURT.height / 2; this.speed = 440;
    this.vx = Math.cos(angle) * this.speed * direction; this.vy = Math.sin(angle) * this.speed;
  }
  move(delta: number): void { this.x += this.vx * delta; this.y += this.vy * delta; }
  rebound(player: Player, paddleCenter: number): void {
    const offset = Math.max(-1, Math.min(1, (this.y - paddleCenter) / (COURT.paddleHeight / 2)));
    this.speed = Math.min(980, this.speed + 28);
    this.vx = Math.cos(offset * 1.05) * this.speed * (player === 0 ? 1 : -1);
    this.vy = Math.sin(offset * 1.05) * this.speed;
    const face = player === 0 ? COURT.paddleInset + COURT.paddleWidth : COURT.width - COURT.paddleInset - COURT.paddleWidth;
    this.x = face + (player === 0 ? 10 : -10);
  }
  render(context: CanvasRenderingContext2D, reducedMotion: boolean): void {
    context.fillStyle = '#fff'; context.shadowColor = '#bcfaff'; context.shadowBlur = reducedMotion ? 0 : 22;
    context.beginPath(); context.arc(this.x, this.y, 9, 0, Math.PI * 2); context.fill(); context.shadowBlur = 0;
  }
}

export class Drone {
  private static readonly sprite = ['11000000011', '00100100100', '11111111111', '00122222100', '00021212000', '00022222000', '00010001000'];
  public x = 0; public y = 0;
  private baseY = 0; private offset = 0; private cooldown = 0;
  constructor(private readonly slot: number) { this.respawn(); }
  get active(): boolean { return this.cooldown <= 0; }
  respawn(): void { this.x = 300 + this.slot * 300; this.y = this.baseY = 150 + Math.random() * 320; this.offset = Math.random() * Math.PI * 2; this.cooldown = 0; }
  destroy(): void { this.cooldown = 5; }
  update(delta: number, elapsed: number, reducedMotion: boolean): void {
    if (!this.active) { this.cooldown -= delta; if (this.cooldown <= 0) this.respawn(); return; }
    if (!reducedMotion) this.y = this.baseY + Math.sin(elapsed * 1.8 + this.offset) * 35;
  }
  hitBy(ball: Ball): boolean { return this.active && Math.abs(ball.x - this.x) < 31 && Math.abs(ball.y - this.y) < 23; }
  render(context: CanvasRenderingContext2D): void {
    if (!this.active) return;
    const pixel = 4, left = Math.round(this.x - 22), top = Math.round(this.y - 14);
    Drone.sprite.forEach((row, rowIndex) => [...row].forEach((cell, columnIndex) => {
      if (cell === '0') return; context.fillStyle = cell === '1' ? '#ffd66b' : '#b486ee'; context.fillRect(left + columnIndex * pixel, top + rowIndex * pixel, pixel, pixel);
    }));
  }
}

export class Particle {
  public life = .5;
  constructor(public x: number, public y: number, private readonly vx: number, private readonly vy: number, private readonly color: string) {}
  update(delta: number): boolean { this.life -= delta; this.x += this.vx * delta; this.y += this.vy * delta; return this.life > 0; }
  render(context: CanvasRenderingContext2D): void { context.globalAlpha = Math.max(0, this.life * 2); context.fillStyle = this.color; context.fillRect(this.x, this.y, 3, 3); }
}

export class BonusPopup {
  public life = 1.3;
  constructor(private readonly x: number, private readonly y: number, private readonly player: Player) {}
  update(delta: number): boolean { this.life -= delta; return this.life > 0; }
  render(context: CanvasRenderingContext2D, reducedMotion: boolean): void {
    context.globalAlpha = Math.min(1, this.life * 2); context.fillStyle = this.player === 0 ? '#64f8e4' : '#ff689e'; context.font = 'bold 20px monospace'; context.textAlign = 'center';
    context.fillText('+1 DRONE', this.x, this.y - 24 - (reducedMotion ? 0 : (1.3 - this.life) * 35));
  }
}
