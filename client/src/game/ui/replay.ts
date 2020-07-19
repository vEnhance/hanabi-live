// Functions for progressing forward and backward through time

import Konva from 'konva';
import * as arrows from './arrows';
import Shuttle from './controls/Shuttle';
import globals from './globals';
import { animate } from './konvaHelpers';

// ---------------------
// Main replay functions
// ---------------------

export const enter = (customSegment?: number) => {
  // Local variables
  const state = globals.store!.getState();

  if (state.replay.active) {
    return;
  }

  // By default, use the final segment of the ongoing game, or 0
  const segment = customSegment ?? state.ongoingGame.turn.segment ?? 0;

  globals.store!.dispatch({
    type: 'replayEnter',
    segment,
  });
};

export const exit = () => {
  if (!globals.store!.getState().replay.active) {
    return;
  }

  // Always animate fast if we are exiting a replay, even if we are only jumping to an adjacent turn
  globals.store!.dispatch({
    type: 'replayExit',
  });
};

export const getCurrentReplaySegment = () => {
  const state = globals.store!.getState();
  const finalSegment = state.ongoingGame.turn.segment!;
  return state.replay.active ? state.replay.segment : finalSegment;
};

export const goToSegment = (
  segment: number,
  breakFree: boolean = false,
  force: boolean = false,
) => {
  // Local variables
  const state = globals.store!.getState();
  const finalSegment = state.ongoingGame.turn.segment!;
  const currentSegment = getCurrentReplaySegment();

  // Validate the target segment
  // The target must be between 0 and the final replay segment
  const clamp = (n: number, min: number, max: number) => Math.max(min, Math.min(n, max));
  const newSegment = clamp(segment, 0, finalSegment);
  if (currentSegment === newSegment) {
    return;
  }

  // Disable replay navigation while we are in a hypothetical
  // (hypothetical navigation functions will set "force" equal to true)
  if (globals.hypothetical && !force) {
    return;
  }

  // Enter the replay, if we are not already
  enter(newSegment);

  // By default, most replay navigation actions should "break free" from the shared segments to
  // allow users to go off on their own side adventure through the game
  // However, if we are navigating to a new segment as the shared replay leader,
  // do not disable shared segments
  if (
    globals.sharedReplay
    && breakFree
    && state.replay.useSharedSegments
    && !globals.amSharedReplayLeader
  ) {
    globals.store!.dispatch({
      type: 'replayUseSharedSegments',
      useSharedSegments: false,
    });
  }

  globals.store!.dispatch({
    type: 'replaySegment',
    segment: newSegment,
  });

  if (globals.sharedReplay && globals.amSharedReplayLeader && state.replay.useSharedSegments) {
    globals.store!.dispatch({
      type: 'replaySharedSegment',
      segment: newSegment,
    });
  }
};

export const goToSegmentAndIndicateCard = (segment: number, order: number) => {
  goToSegment(segment, true);

  // We indicate the card to make it easier to see
  arrows.hideAll(); // We hide all the arrows first to ensure that the arrow is always shown
  arrows.toggle(globals.deck[order]);
};

// ---------------------------
// Replay navigation functions
// ---------------------------

export const back = (breakFree: boolean = true) => {
  goToSegment(getCurrentReplaySegment() - 1, breakFree);
};

export const forward = () => {
  goToSegment(getCurrentReplaySegment() + 1, true);
};

export const backRound = () => {
  goToSegment(getCurrentReplaySegment() - globals.options.numPlayers, true);
};

export const forwardRound = () => {
  goToSegment(getCurrentReplaySegment() + globals.options.numPlayers, true);
};

export const backFull = () => {
  goToSegment(0, true);
};

export const forwardFull = () => {
  const finalSegment = globals.store!.getState().ongoingGame.turn.segment!;
  goToSegment(finalSegment, true);
};

// ------------------------
// The "Exit Replay" button
// ------------------------

export const exitButton = () => {
  // Mark the time that the user clicked the "Exit Replay" button
  // (so that we can avoid an accidental "Give Clue" double-click)
  globals.UIClickTime = Date.now();

  exit();
};

// ------------------
// The replay shuttle
// ------------------

export function barClick(this: Konva.Rect) {
  const rectX = globals.stage.getPointerPosition().x - this.getAbsolutePosition().x;
  const w = this.width();
  const finalSegment = globals.store!.getState().ongoingGame.turn.segment!;
  const step = w / finalSegment;
  const newSegment = Math.floor((rectX + (step / 2)) / step);
  goToSegment(newSegment, true);
}

export function barDrag(this: Konva.Rect, pos: Konva.Vector2d) {
  const min = globals.elements.replayBar!.getAbsolutePosition().x + (this.width() * 0.5);
  const w = globals.elements.replayBar!.width() - this.width();
  let shuttleX = pos.x - min;
  const shuttleY = this.getAbsolutePosition().y;
  if (shuttleX < 0) {
    shuttleX = 0;
  }
  if (shuttleX > w) {
    shuttleX = w;
  }
  const finalSegment = globals.store!.getState().ongoingGame.turn.segment!;
  const step = w / finalSegment;
  const newSegment = Math.floor((shuttleX + (step / 2)) / step);
  goToSegment(newSegment, true);
  shuttleX = newSegment * step;
  return {
    x: min + shuttleX,
    y: shuttleY,
  };
}

const positionReplayShuttle = (
  shuttle: Shuttle,
  targetSegment: number,
  smaller: boolean,
  fast: boolean,
) => {
  let finalSegment = globals.store!.getState().ongoingGame.turn.segment;
  if (
    finalSegment === null // The final segment is null during initialization
    || finalSegment === 0 // The final segment is 0 before a move is made
  ) {
    // For the purposes of the replay shuttle calculation,
    // we need to assume that there are at least two possible locations
    finalSegment = 1;
  }
  const winH = globals.stage.height();
  const sliderW = globals.elements.replayBar!.width() - shuttle.width();
  const x = (
    globals.elements.replayBar!.x()
    + (sliderW / finalSegment * targetSegment)
    + (shuttle.width() / 2)
  );
  let y = globals.elements.replayBar!.y() + (shuttle.height() * 0.55);
  if (smaller) {
    y -= 0.003 * winH;
  }
  const scale = smaller ? 0.7 : 1;
  animate(shuttle, {
    duration: 0.25,
    x,
    y,
    scale,
    easing: Konva.Easings.EaseOut,
  }, true, fast);
};

export const adjustShuttles = (fast: boolean) => {
  // Local variables
  const state = globals.store!.getState();

  // If the two shuttles are overlapping, then make the normal shuttle a little bit smaller
  let smaller = false;
  if (
    globals.sharedReplay
    && !state.replay.useSharedSegments
    && state.replay.segment === state.replay.sharedSegment
  ) {
    smaller = true;
  }

  // Adjust the shuttles along the replay bar based on the current segment
  // If it is smaller, we need to nudge it to the right a bit in order to center it
  positionReplayShuttle(
    globals.elements.replayShuttleShared!,
    state.replay.sharedSegment,
    false,
    fast,
  );
  positionReplayShuttle(
    globals.elements.replayShuttle!,
    state.replay.segment,
    smaller,
    fast,
  );
};

// -----------------------------
// Right-clicking the turn count
// -----------------------------

export const promptTurn = () => {
  const turnString = window.prompt('Which turn do you want to go to?');
  if (turnString === null) {
    return;
  }
  let targetTurn = parseInt(turnString, 10);
  if (Number.isNaN(targetTurn)) {
    return;
  }

  // We need to decrement the turn because
  // the turn shown to the user is always one greater than the real turn
  targetTurn -= 1;

  goToSegment(targetTurn, true);
};

// --------------------------------
// The "Toggle Shared Turns" button
// --------------------------------

export const toggleSharedSegments = () => {
  // Local variables
  const state = globals.store!.getState();

  // If we are the replay leader and we are re-enabling shared segments,
  // first update the shared segment to our current segment
  if (globals.amSharedReplayLeader && !state.replay.useSharedSegments) {
    globals.store!.dispatch({
      type: 'replaySharedSegment',
      segment: getCurrentReplaySegment(),
    });
  }

  globals.store!.dispatch({
    type: 'replayUseSharedSegments',
    useSharedSegments: !state.replay.useSharedSegments,
  });
};
