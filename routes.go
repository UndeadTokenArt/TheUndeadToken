package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"os"

	"github.com/undeadtokenart/Homepage/internal/hub"
	"github.com/undeadtokenart/Homepage/internal/store"
)

// Global variables
var (
	wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	st         = store.New()
	hb         = hub.New()
)

// uidFromCookie retrieves the UID from the cookie or generates a new one
func uidFromCookie(c *gin.Context) string {
	if v, err := c.Cookie("uid"); err == nil && v != "" {
		return v
	}

	// generate new UID based on timestamp
	v := time.Now().Format("20060102150405.000000000")
	c.SetCookie("uid", v, 86400*365, "/", "", false, true)
	return v
}

// registerRoutes sets up the HTTP routes for the Gin engine
func registerRoutes(r *gin.Engine) {

	// Homepage route
	r.GET("/", func(c *gin.Context) {
		// Load config data
		homepage, err := ParseConfigFile("config.json")
		if err != nil {
			log.Printf("Error loading config: %v", err)
			c.HTML(500, "index.tmpl", gin.H{
				"error": "Failed to load configuration",
			})
			return
		}

		// Render template with config data
		c.HTML(200, "index.tmpl", homepage)
	})

	// GM Tools route
	r.GET("/gmTools", func(c *gin.Context) {
		c.HTML(http.StatusOK, "gmTools.tmpl", gin.H{})
	})

	// Quests route
	r.GET("/quests", func(c *gin.Context) {
		data, err := os.ReadFile("static/quests/questTest.json")
		if err != nil {
			log.Printf("Error reading quest file: %v", err)
			c.HTML(http.StatusInternalServerError, "quests.tmpl", gin.H{"Quests": nil})
			return
		}

		var qf struct {
			Quests []struct {
				Name        string   `json:"name"`
				Description string   `json:"description"`
				Reward      string   `json:"reward"`
				Giver       string   `json:"Giver"`
				Level       int      `json:"level"`
				Tags        []string `json:"tags"`
			} `json:"quests"`
		}
		if err := json.Unmarshal(data, &qf); err != nil {
			log.Printf("Error unmarshaling quest JSON: %v", err)
			c.HTML(http.StatusInternalServerError, "quests.tmpl", gin.H{"Quests": nil})
			return
		}

		type questView struct {
			Title       string
			Name        string
			Level       int
			Description string
			Giver       string
			Reward      string
		}

		var questViews []questView
		for _, q := range qf.Quests {
			questViews = append(questViews, questView{
				Title:       q.Name,
				Name:        "Quest",
				Level:       q.Level,
				Description: q.Description,
				Giver:       q.Giver,
				Reward:      q.Reward,
			})
		}

		c.HTML(http.StatusOK, "quests.tmpl", gin.H{"Quests": questViews})
	})

	// Join or create group route
	r.POST("/join", func(c *gin.Context) {
		uid := uidFromCookie(c)
		code := strings.ToUpper(strings.TrimSpace(c.PostForm("code")))
		if code == "" {
			// create new
			g := st.CreateOrGetGroup("", uid)
			c.Redirect(http.StatusSeeOther, "/g/"+g.Code)
			return
		}
		_ = st.CreateOrGetGroup(code, uid)
		c.Redirect(http.StatusSeeOther, "/g/"+code)
	})

	// Group page route
	r.GET("/g/:code", func(c *gin.Context) {
		uid := uidFromCookie(c)
		code := strings.ToUpper(c.Param("code"))
		g, ok := st.GetGroup(code)
		if !ok {
			c.String(http.StatusNotFound, "group not found")
			return
		}
		isDM := g.DMUID == uid
		c.HTML(http.StatusOK, "group.tmpl", gin.H{"Code": code, "IsDM": isDM})
	})

	// WebSocket route for real-time updates
	r.GET("/ws/:code", func(c *gin.Context) {
		uid := uidFromCookie(c)
		code := strings.ToUpper(c.Param("code"))
		g, ok := st.GetGroup(code)
		if !ok {
			c.String(http.StatusNotFound, "group not found")
			return
		}
		isDM := g.DMUID == uid
		conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		client := &hub.Client{Conn: conn, UID: uid, IsDM: isDM, Group: code, SendCh: make(chan []byte, 8)}
		hb.AddClient(code, client)

		// writer
		go func() {
			for msg := range client.SendCh {
				_ = conn.WriteMessage(websocket.TextMessage, msg)
			}
		}()

		// initial state
		hb.BroadcastState(code, g)

		// reader
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}

			// Minimal message router
			// Expecting JSON messages with "type" and "data"
			type Incoming struct {
				Type string                 `json:"type"`
				Data map[string]interface{} `json:"data"`
			}
			var in Incoming
			if err := json.Unmarshal(data, &in); err != nil {
				continue
			}
			switch in.Type {

			// Handle different message types
			case "addPlayer":
				name := strings.TrimSpace(getStr(in.Data, "name"))
				init := getInt(in.Data, "initiative")
				bonus := getInt(in.Data, "bonus")
				if name == "" {
					name = "Player"
				}
				st.AddPlayer(code, uid, name, init, bonus)
			case "addPlayerRoll":
				name := strings.TrimSpace(getStr(in.Data, "name"))
				bonus := getInt(in.Data, "bonus")
				if name == "" {
					name = "Player"
				}
				st.AddPlayerWithRoll(code, uid, name, bonus)
			case "addMonster":
				if !isDM {
					break
				}
				name := strings.TrimSpace(getStr(in.Data, "name"))
				hp := getInt(in.Data, "hp")
				init := getInt(in.Data, "initiative")
				bonus := getInt(in.Data, "bonus")
				if name == "" {
					name = "Monster"
				}
				st.AddMonster(code, uid, name, hp, bonus, init)
			case "damage":
				if !isDM {
					break
				}
				id := getStr(in.Data, "id")
				dmg := getInt(in.Data, "dmg")
				st.DamageMonster(code, uid, id, dmg)
			case "reorder":
				if !isDM {
					break
				}
				order := getStringSlice(in.Data, "order")
				st.Reorder(code, uid, order)
			case "next":
				st.NextTurn(code)
			case "reset":
				if !isDM {
					break
				}
				st.ResetInitiative(code, uid)
			case "deleteEntity":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				st.DeleteEntity(code, uid, entityID)
			case "renameEntity":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				newName := strings.TrimSpace(getStr(in.Data, "name"))
				if newName != "" {
					st.RenameEntity(code, uid, entityID, newName)
				}
			case "editEntityHP":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				currentHP := getInt(in.Data, "hp")
				maxHP := getInt(in.Data, "maxHp")
				st.EditEntityHP(code, uid, entityID, currentHP, maxHP)
			case "addEntityTag":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				tag := strings.TrimSpace(getStr(in.Data, "tag"))
				if tag != "" {
					st.AddEntityTag(code, uid, entityID, tag)
				}
			case "removeEntityTag":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				tag := strings.TrimSpace(getStr(in.Data, "tag"))
				if tag != "" {
					st.RemoveEntityTag(code, uid, entityID, tag)
				}
			}
			if g2, ok := st.GetGroup(code); ok {
				hb.BroadcastState(code, g2)
			}
		}
		hb.RemoveClient(code, client)
		_ = conn.Close()
	})
}

// Helper functions to extract typed values from maps
func getStr(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getInt extracts an integer from a map, handling different possible types
func getInt(m map[string]interface{}, k string) int {
	if v, ok := m[k]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case string:
			// ignore parse for brevity
		}
	}
	return 0
}

// getStringSlice extracts a slice of strings from a map
func getStringSlice(m map[string]interface{}, k string) []string {
	var out []string
	if v, ok := m[k]; ok {
		if arr, ok := v.([]interface{}); ok {
			for _, x := range arr {
				if s, ok := x.(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	return out
}
