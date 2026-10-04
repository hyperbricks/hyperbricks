import type { Player } from './types';

interface AudioWindow extends Window { webkitAudioContext?: typeof AudioContext; }

export class SoundEngine {
  private context?: AudioContext;
  public enabled = true;

  unlock(): void {
    try {
      const Constructor = window.AudioContext || (window as AudioWindow).webkitAudioContext;
      if (!Constructor) return;
      this.context ??= new Constructor();
      void this.context.resume().catch(() => undefined);
    } catch { /* Sound is progressive enhancement. */ }
  }

  tone(frequency: number, duration = .09, type: OscillatorType = 'square', end = frequency, delay = 0, volume = .055): void {
    if (!this.enabled || !this.context || this.context.state !== 'running') return;
    const oscillator = this.context.createOscillator(), gain = this.context.createGain(), start = this.context.currentTime + delay;
    oscillator.type = type; oscillator.frequency.setValueAtTime(frequency, start); oscillator.frequency.exponentialRampToValueAtTime(Math.max(20, end), start + duration);
    gain.gain.setValueAtTime(volume, start); gain.gain.exponentialRampToValueAtTime(.001, start + duration);
    oscillator.connect(gain); gain.connect(this.context.destination); oscillator.start(start); oscillator.stop(start + duration);
    oscillator.onended = () => { oscillator.disconnect(); gain.disconnect(); };
  }
  serve(): void { this.tone(550, .1, 'triangle', 850); }
  paddle(player: Player, rally: number): void { this.tone((player === 0 ? 480 : 650) + Math.min(300, rally * 15), .08, 'square', 240); }
  wall(): void { this.tone(240, .055, 'triangle', 180); }
  score(): void { this.tone(180, .25, 'sawtooth', 65, 0, .07); }
  drone(): void { this.tone(880, .12, 'square', 1320); this.tone(1320, .16, 'triangle', 1760, .09, .08); }
  match(won: boolean): void { (won ? [330, 440, 554, 880] : [330, 277, 220, 110]).forEach((note, index) => this.tone(note, .23, 'triangle', note, index * .12, .12)); }
}
