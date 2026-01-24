// Package models provides core data structures and logic for managing entities and groups
// in a turn-based initiative system, such as those used in tabletop role-playing games.
// It defines types for entities (players and monsters), groups that organize entities,
// and utility functions for sorting entities by initiative, advancing turns and rounds,
// and rolling dice. The package ensures stable and fair ordering of turns, supports
// initiative bonuses, and tracks health and ownership for each entity.
package models

import (
	"math/rand"
	"sort"
	"time"
)




type EntityType string

const (
	Player  EntityType = "player"
	Monster EntityType = "monster"
)


type Entity struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       EntityType `json:"type"`
	Initiative int        `json:"initiative"`
	Bonus      int        `json:"bonus"`
	HP         int        `json:"hp"`
	MaxHP      int        `json:"maxHp"`
	OwnerUID   string     `json:"ownerUid"` // for players
	Tags       []string   `json:"tags"`     // for conditions like "poisoned", "stunned", etc.
}

type Group struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"createdAt"`
	DMUID     string    `json:"dmUid"`
	Round     int       `json:"round"`
	TurnIndex int       `json:"turnIndex"`
	Entities  []Entity  `json:"entities"`
}

// SortOrder sorts by initiative desc; players before monsters on tie; stable by name.
func (g *Group) SortOrder() {
	sort.SliceStable(g.Entities, func(i, j int) bool {
		a, b := g.Entities[i], g.Entities[j]
		if a.Initiative != b.Initiative {
			return a.Initiative > b.Initiative
		}
		if a.Type != b.Type {
			return a.Type == Player // players before monsters on tie
		}
		return a.Name < b.Name
	})
}

func (g *Group) NextTurn() {
	if len(g.Entities) == 0 {
		g.TurnIndex = 0
		g.Round = 1
		return
	}
	g.TurnIndex++
	if g.TurnIndex >= len(g.Entities) {
		g.TurnIndex = 0
		if g.Round == 0 {
			g.Round = 1
		} else {
			g.Round++
		}
	}
}

// RollD20 returns 1..20
func RollD20() int {
	return rand.Intn(20) + 1
}
