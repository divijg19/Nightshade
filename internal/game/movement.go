package game

import (
    "github.com/divijg19/Nightshade/internal/agent"
    "github.com/divijg19/Nightshade/internal/world"
)

// ResolveMovement is a pure function that computes the new position resulting
// from applying `action` to `pos`. It enforces world bounds: if the target
// would be outside the world bounds, the original position is returned.
func ResolveMovement(pos world.Position, action agent.Action) world.Position {
    var tgt world.Position
    switch action {
    case agent.MOVE_N:
        tgt = world.Position{X: pos.X, Y: pos.Y - 1}
    case agent.MOVE_S:
        tgt = world.Position{X: pos.X, Y: pos.Y + 1}
    case agent.MOVE_E:
        tgt = world.Position{X: pos.X + 1, Y: pos.Y}
    case agent.MOVE_W:
        tgt = world.Position{X: pos.X - 1, Y: pos.Y}
    default:
        return pos
    }

    // Enforce bounds: if out-of-bounds, return original position.
    if tgt.X < 0 || tgt.Y < 0 || tgt.X >= world.Width || tgt.Y >= world.Height {
        return pos
    }
    return tgt
}
