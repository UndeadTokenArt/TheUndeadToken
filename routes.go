package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/undeadtokenart/Homepage/internal/hub"
	"github.com/undeadtokenart/Homepage/internal/store"
)

var (
	wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	st         = store.New()
	hb         = hub.New()
)

func uidFromCookie(c *gin.Context) string {
	if v, err := c.Cookie("uid"); err == nil && v != "" {
		return v
	}
	v := time.Now().Format("20060102150405.000000000")
	c.SetCookie("uid", v, 86400*365, "/", "", false, true)
	return v
}

func registerRoutes(r *gin.Engine) {
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

	r.GET("/gmTools", func(c *gin.Context) {
		c.HTML(http.StatusOK, "gmTools.tmpl", gin.H{})
	})

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

	// WebSocket endpoint
	r.GET("/ws/:code", func(c *gin.Context) {
		uid := uidFromCookie(c)
		code := strings.ToUpper(c.Param("code"))

		g, ok := st.GetGroupSnapshot(code)
		if !ok {
			c.String(http.StatusNotFound, "group not found")
			return
		}

		isDM := g.DMUID == uid
		conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := &hub.Client{
			Conn:   conn,
			UID:    uid,
			IsDM:   isDM,
			Group:  code,
			SendCh: make(chan []byte, 16),
			Done:   make(chan struct{}),
		}

		hb.AddClient(code, client)

		defer func() {
			hb.RemoveClient(code, client)
			close(client.Done)
			_ = conn.Close()
		}()

		go func() {
			for {
				select {
				case msg := <-client.SendCh:
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						return
					}
				case <-client.Done:
					return
				}
			}
		}()

		hb.BroadcastState(code, g)

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				break
			}

			type Incoming struct {
				Type string                 `json:"type"`
				Data map[string]interface{} `json:"data"`
			}

			var in Incoming
			if err := json.Unmarshal(data, &in); err != nil {
				continue
			}

			switch in.Type {
			case "ping":
				select {
				case client.SendCh <- []byte(`{"type":"pong"}`):
				default:
					return
				}
			case "addPlayer":
				name := strings.TrimSpace(getStr(in.Data, "name"))
				init := getInt(in.Data, "initiative")
				bonus := getInt(in.Data, "bonus")
				if name == "" {
					name = "Player"
				}
				_, _, _ = st.AddPlayer(code, uid, name, init, bonus)
			case "addPlayerRoll":
				name := strings.TrimSpace(getStr(in.Data, "name"))
				bonus := getInt(in.Data, "bonus")
				if name == "" {
					name = "Player"
				}
				_, _, _ = st.AddPlayerWithRoll(code, uid, name, bonus)
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
				_, _, _ = st.AddMonster(code, uid, name, hp, bonus, init)
			case "damage":
				if !isDM {
					break
				}
				id := getStr(in.Data, "id")
				dmg := getInt(in.Data, "dmg")
				_, _ = st.DamageMonster(code, uid, id, dmg)
			case "reorder":
				if !isDM {
					break
				}
				order := getStringSlice(in.Data, "order")
				_, _ = st.Reorder(code, uid, order)
			case "next":
				_, _ = st.NextTurn(code)
			case "reset":
				if !isDM {
					break
				}
				_, _ = st.ResetInitiative(code, uid)
			case "deleteEntity":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				_, _ = st.DeleteEntity(code, uid, entityID)
			case "renameEntity":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				newName := strings.TrimSpace(getStr(in.Data, "name"))
				if newName != "" {
					_, _ = st.RenameEntity(code, uid, entityID, newName)
				}
			case "editEntityHP":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				currentHP := getInt(in.Data, "hp")
				maxHP := getInt(in.Data, "maxHp")
				_, _ = st.EditEntityHP(code, uid, entityID, currentHP, maxHP)
			case "addEntityTag":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				tag := strings.TrimSpace(getStr(in.Data, "tag"))
				if tag != "" {
					_, _ = st.AddEntityTag(code, uid, entityID, tag)
				}
			case "removeEntityTag":
				if !isDM {
					break
				}
				entityID := getStr(in.Data, "id")
				tag := strings.TrimSpace(getStr(in.Data, "tag"))
				if tag != "" {
					_, _ = st.RemoveEntityTag(code, uid, entityID, tag)
				}
			}

			g2, ok := st.GetGroupSnapshot(code)
			if ok {
				hb.BroadcastState(code, g2)
			}
		}
	})
}

func getStr(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
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
