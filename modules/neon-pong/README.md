# Neon Rally — HyperBricks Pong

A self-contained vibecoded arcade Pong module example for HyperBricks v1.2.8-beta and later.

## Play

Run from the project root (the folder containing `modules/`):

```sh
hyperbricks start -m neon-pong --port 8080 --non-interactive
```

Open http://localhost:8080/ and press **Let’s Play**. The first player to seven wins.

The game runs without a developer login. Its dashboard is disabled by default.
If you enable the developer interface in `package.hyperbricks.yaml`, set
credentials before startup; there is no built-in account:

```sh
export HB_DEVELOPER_USER=developer
export HB_DEVELOPER_PASSWORD='choose-a-long-password'
```

- **Solo:** W/S or Up/Down move your paddle. Choose Chill, Arcade or Expert CPU difficulty.
- **Two players:** player one uses W/S; player two uses Up/Down.
- **Touch/mouse:** drag on the court. In two-player mode each half controls its own paddle; simultaneous touch pointers are supported.
- **Pause:** Space or the Pause button. Escape toggles pause/resume. Switching away pauses the match automatically.
- **Reset:** returns to the ready screen and clears scores. Changing modes starts a fresh match.
- **Sound:** toggle Sound On/Off. Audio starts after a Play or sound-button gesture, as required by browsers.

## Drone bonuses

Three pixel drones hover around the court. Hit a drone with the ball for **+1 point** awarded to the player who last hit the ball, including the CPU. Bonuses keep the rally going and can win the match. Drones respawn after five seconds; fresh serves earn no drone points until a paddle hits the ball. A colored +1 popup, pixel explosion and bonus sound confirm each hit. Drone movement freezes while paused and stays still with reduced motion enabled.

## Features

Neon court, glowing paddles, ball trails, impact sparks, short screen shakes, angled paddle rebounds, and progressively faster rallies. Synthesized sounds distinguish paddle hits, wall hits, scoring, serves, and match results. Reduced-motion preferences suppress trails, particles and shakes. Responsive controls, visible keyboard focus and score announcements are included.

The running game has no network dependency, sound downloads or Go plugins. All visuals are CSS and canvas; effects use the Web Audio API. Native HyperBricks esbuild transpiles the TypeScript source and produces fingerprinted local JavaScript.

TypeScript validation is optional; the game itself builds through HyperBricks
without npm. To run the pinned check, use `npm ci` and `npm run check` from
`modules/neon-pong`.

## Source

- `hyperbricks/pong.hyperbricks.yaml`: full-document root route and template composition.
- `hyperbricks/partials/assets.hyperbricks.yaml`: native esbuild asset components.
- `templates/pong.html`: game interface.
- `resources/css/app.css`: responsive arcade styling.
- `resources/js/app.ts`: small composition root that starts the game.
- `resources/js/game.ts`: `PongGame` application controller, rules, input, UI state and rendering orchestration.
- `resources/js/entities.ts`: `Ball`, `Drone`, `Particle` and `BonusPopup` domain classes.
- `resources/js/sound-engine.ts`: Web Audio synthesis service.
- `resources/js/types.ts`: shared game contracts and court constants.
- `package.hyperbricks.yaml`: module directories and development settings.

## Verification

Verified startup and HTTP rendering with HyperBricks v1.2.9-beta. Browser checks cover rally progression, keyboard pause, two-player selection, the sound toggle and console errors. A local simulation verified first-to-seven completion, pause freezing scores, reset, audio synthesis calls and touch input handling. Audible playback and physical multitouch should also be checked on your target device.
