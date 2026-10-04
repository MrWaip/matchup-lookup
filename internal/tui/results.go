package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"matchup-lookup/internal/core"
)

func PrintResults(results []core.Result, players, total, matching, wins int) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	fmt.Println(title.Render("◆ MATCHUP LOOKUP"))
	fmt.Printf("Players %d  ·  Stored %d  ·  Matching %d  ·  %s  ·  %s\n\n",
		players, total, matching, green.Render(fmt.Sprintf("Wins %d", wins)), red.Render(fmt.Sprintf("Losses %d", matching-wins)))
	if len(results) == 0 {
		fmt.Println(muted.Render("No games found. Try -days 0 or a broader rank filter."))
		return
	}
	for i, r := range results {
		printResult(i+1, r)
		if i+1 < len(results) {
			fmt.Println()
		}
	}
}

func printResult(number int, r core.Result) {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	resultStyle := red
	status := "LOSS"
	if r.Win {
		resultStyle = green
		status = "WIN"
	}
	fmt.Printf("%d. %s  %s vs %s  ·  %s  ·  %s  ·  patch %s\n",
		number, resultStyle.Bold(true).Render(status), r.Champion, r.Opponent, r.Date.Format("02 Jan 15:04"), r.Region, r.Patch)
	fmt.Printf("   Player Riot ID: %s\n", title.Render(r.PlayerID))
	fmt.Printf("   Rank %s  ·  K/D/A %s  ·  CS %d\n", r.Rank, r.KDA, r.CS)
	fmt.Printf("   Opponent Riot ID: %s  ·  K/D/A %s\n", r.OpponentID, r.OpponentKDA)
	fmt.Printf("   Spells: %s  ·  Keystone: %s  ·  Secondary: %s\n", r.Spells, r.Keystone, r.SecondaryRunes)
	fmt.Printf("   Match ID: %s", r.MatchID)
	if r.Status != "confirmed" {
		fmt.Printf("  ·  position %s", r.Status)
	}
	fmt.Println()
}
