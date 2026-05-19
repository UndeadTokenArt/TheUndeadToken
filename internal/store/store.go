// Package store provides an in-memory data store for managing groups and entities
// in the AgentTracker application. It supports concurrent access and operations
// such as creating groups, adding players and monsters, tracking initiative order,
// applying damage, reordering entities, advancing turns, and resetting group state.
//
// The Store type maintains a map of groups, each identified by a unique code.
// Groups contain entities representing players and monsters, with initiative and
// other combat-related attributes. The store ensures thread-safe access using
// sync.RWMutex.
//
// Key features:
//   - Create or retrieve groups with unique codes
//   - Add players (with or without initiative rolls) and monsters to groups
//   - Sort entities by initiative order
//   - Apply damage to monsters
//   - Reorder entities within a group
//   - Advance to the next turn and reset initiative for a group
//   - Enforce DM (Dungeon Master) permissions for sensitive operations
//
// This package is intended for use as the backend state management for turn-based
// combat tracking in tabletop RPG scenarios.
package store

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/undeadtokenart/Homepage/internal/models"
)

const defaultDataPath = "groups/state.json"

type persistedState struct {
	Groups map[string]*models.Group `json:"groups"`
}

type Store struct {
	mu       sync.RWMutex
	groups   map[string]*models.Group
	dataPath string
}

func New() *Store {
	dataPath := os.Getenv("STORE_FILE")
	if dataPath == "" {
		dataPath = defaultDataPath
	}

	s := &Store{groups: make(map[string]*models.Group), dataPath: dataPath}
	if err := s.loadFromDisk(); err != nil {
		log.Printf("failed to load store from %s: %v", s.dataPath, err)
	}
	return s
}

func (s *Store) loadFromDisk() error {
	b, err := os.ReadFile(s.dataPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var state persistedState
	if err := json.Unmarshal(b, &state); err != nil {
		return err
	}
	if state.Groups == nil {
		state.Groups = make(map[string]*models.Group)
	}
	s.groups = state.Groups
	return nil
}

func (s *Store) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.dataPath), 0o755); err != nil {
		return err
	}

	b, err := json.Marshal(persistedState{Groups: s.groups})
	if err != nil {
		return err
	}

	tmp := s.dataPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.dataPath)
}

func (s *Store) persistLockedLog() {
	if err := s.persistLocked(); err != nil {
		log.Printf("failed to persist store to %s: %v", s.dataPath, err)
	}
}

func randomCode() string {
	letters := []rune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
	b := make([]rune, 5)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (s *Store) CreateOrGetGroup(code, uid string) *models.Group {
	s.mu.Lock()
	defer s.mu.Unlock()
	if code == "" {
		code = randomCode()
	}
	g, ok := s.groups[code]
	if !ok {
		g = &models.Group{Code: code, CreatedAt: time.Now(), DMUID: uid, Round: 1}
		s.groups[code] = g
	}
	if g.DMUID == "" {
		g.DMUID = uid
	}
	s.persistLockedLog()
	return g
}

func (s *Store) GetGroup(code string) (*models.Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[code]
	return g, ok
}

func (s *Store) GetGroupSnapshot(code string) (*models.Group, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	g, ok := s.groups[code]
	if !ok {
		return nil, false
	}

	clone := *g
	clone.Entities = make([]models.Entity, len(g.Entities))
	for i, entity := range g.Entities {
		entityCopy := entity
		if entity.Tags != nil {
			entityCopy.Tags = append([]string(nil), entity.Tags...)
		}
		clone.Entities[i] = entityCopy
	}

	return &clone, true
}

func (s *Store) AddPlayer(code, uid, name string, initiative, bonus int) (*models.Group, models.Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, models.Entity{}, errors.New("group not found")
	}
	if initiative < 0 {
		initiative = 0
	}
	e := models.Entity{ID: uuid.NewString(), Name: name, Type: models.Player, Initiative: initiative, Bonus: bonus, OwnerUID: uid}
	g.Entities = append(g.Entities, e)
	g.SortOrder()
	s.persistLockedLog()
	return g, e, nil
}

func (s *Store) AddPlayerWithRoll(code, uid, name string, bonus int) (*models.Group, models.Entity, error) {
	roll := models.RollD20()
	return s.AddPlayer(code, uid, name, roll+bonus, bonus)
}

func (s *Store) AddMonster(code, uid, name string, hp, bonus, initiative int) (*models.Group, models.Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, models.Entity{}, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, models.Entity{}, errors.New("not dm")
	}
	if initiative < 0 {
		initiative = 0
	}
	e := models.Entity{ID: uuid.NewString(), Name: name, Type: models.Monster, Initiative: initiative, Bonus: bonus, HP: hp, MaxHP: hp}
	g.Entities = append(g.Entities, e)
	g.SortOrder()
	s.persistLockedLog()
	return g, e, nil
}

func (s *Store) DamageMonster(code, uid, entityID string, delta int) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	for i := range g.Entities {
		if g.Entities[i].ID == entityID && g.Entities[i].Type == models.Monster {
			g.Entities[i].HP -= delta
			if g.Entities[i].HP < 0 {
				g.Entities[i].HP = 0
			}
			s.persistLockedLog()
			return g, nil
		}
	}
	return nil, errors.New("entity not found")
}

func (s *Store) Reorder(code, uid string, order []string) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	idToEntity := make(map[string]models.Entity)
	for _, e := range g.Entities {
		idToEntity[e.ID] = e
	}
	newList := make([]models.Entity, 0, len(g.Entities))
	for _, id := range order {
		if e, ok := idToEntity[id]; ok {
			newList = append(newList, e)
			delete(idToEntity, id)
		}
	}
	// append any missing
	for _, e := range idToEntity {
		newList = append(newList, e)
	}
	g.Entities = newList
	s.persistLockedLog()
	return g, nil
}

func (s *Store) NextTurn(code string) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	g.NextTurn()
	s.persistLockedLog()
	return g, nil
}

func (s *Store) ResetInitiative(code, uid string) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	// Clear all entities and reset round/turn
	g.Entities = []models.Entity{}
	g.Round = 1
	g.TurnIndex = 0
	s.persistLockedLog()
	return g, nil
}

// DeleteEntity removes an entity from the group (DM only)
func (s *Store) DeleteEntity(code, uid, entityID string) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	for i, entity := range g.Entities {
		if entity.ID == entityID {
			// Remove entity from slice
			g.Entities = append(g.Entities[:i], g.Entities[i+1:]...)
			// Adjust turn index if needed
			if g.TurnIndex > i {
				g.TurnIndex--
			} else if g.TurnIndex >= len(g.Entities) && len(g.Entities) > 0 {
				g.TurnIndex = 0
			}
			s.persistLockedLog()
			return g, nil
		}
	}
	return nil, errors.New("entity not found")
}

// RenameEntity changes an entity's name (DM only)
func (s *Store) RenameEntity(code, uid, entityID, newName string) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	for i := range g.Entities {
		if g.Entities[i].ID == entityID {
			g.Entities[i].Name = newName
			s.persistLockedLog()
			return g, nil
		}
	}
	return nil, errors.New("entity not found")
}

// EditEntityHP modifies an entity's current and max HP (DM only)
func (s *Store) EditEntityHP(code, uid, entityID string, currentHP, maxHP int) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	for i := range g.Entities {
		if g.Entities[i].ID == entityID {
			if currentHP < 0 {
				currentHP = 0
			}
			if maxHP < 0 {
				maxHP = 0
			}
			g.Entities[i].HP = currentHP
			g.Entities[i].MaxHP = maxHP
			s.persistLockedLog()
			return g, nil
		}
	}
	return nil, errors.New("entity not found")
}

// AddEntityTag adds a condition tag to an entity (DM only)
func (s *Store) AddEntityTag(code, uid, entityID, tag string) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	for i := range g.Entities {
		if g.Entities[i].ID == entityID {
			// Check if tag already exists
			for _, existingTag := range g.Entities[i].Tags {
				if existingTag == tag {
					return g, nil // Tag already exists, no need to add
				}
			}
			// Add the new tag
			g.Entities[i].Tags = append(g.Entities[i].Tags, tag)
			s.persistLockedLog()
			return g, nil
		}
	}
	return nil, errors.New("entity not found")
}

// RemoveEntityTag removes a condition tag from an entity (DM only)
func (s *Store) RemoveEntityTag(code, uid, entityID, tag string) (*models.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.groups[code]
	if !ok {
		return nil, errors.New("group not found")
	}
	if g.DMUID != uid {
		return nil, errors.New("not dm")
	}
	for i := range g.Entities {
		if g.Entities[i].ID == entityID {
			// Find and remove the tag
			for j, existingTag := range g.Entities[i].Tags {
				if existingTag == tag {
					g.Entities[i].Tags = append(g.Entities[i].Tags[:j], g.Entities[i].Tags[j+1:]...)
					s.persistLockedLog()
					return g, nil
				}
			}
			return g, nil // Tag not found, but that's okay
		}
	}
	return nil, errors.New("entity not found")
}
