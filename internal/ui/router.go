package ui

func routeView(m Model) string {
	if m.obs.Mode == "board" && m.obs.RunSummary != nil {
		return renderSummary(m)
	}
	if m.obs.Mode == "dungeon" {
		return renderDungeon(m)
	}
	return renderBoard(m)
}
